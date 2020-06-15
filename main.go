/*
Dblfinder provides a command-line tool for finding duplicated files.
When duplicates are found, it can provide an option to delete one of the them.
*/
package main

import (
	"bufio"
	"crypto/md5"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type action string

const (
	version           = "0.5.1"
	KB                = 1024
	keepAction action = "keep"
	listAction action = "list"
)

func getFlags() (action, int, bool, []string, string, string, bool, bool, int) {
	var (
		showHelp, showVersion, skipManual bool
		verbose, dryRun                   bool
		fsLimit, sampleSize               int
		useAction, ignore, prefer         string
		roots                             []string
	)

	flag.BoolVar(&showHelp, "help", false, "display help")
	flag.BoolVar(&showVersion, "version", false, "display the version number")
	flag.BoolVar(&verbose, "verbose", false, "provide verbose output")
	flag.IntVar(&fsLimit, "fs-limit", 100, "limit the maximum number open files")
	flag.StringVar(&useAction, "action", "list", "action to use for duplicates found (list, keep, delete)")
	flag.StringVar(&ignore, "ignore", "", "regexp to ignore files completely")
	flag.StringVar(&prefer, "prefer", "", "regexp to keep files if a duplicate matches it")
	flag.BoolVar(&skipManual, "skip-manual", false, "skip decisions if prefer did not find anything")
	flag.BoolVar(&dryRun, "dry-run", false, "dry run, nothing will be deleted but deletion logic will be executed")
	flag.IntVar(&sampleSize, "sample-size", 1024, "sample size to use for calculating file hashes (KB)")

	flag.Parse()

	roots = flag.Args()

	if showHelp {
		flag.PrintDefaults()
		os.Exit(0)
	}

	if showVersion {
		fmt.Println(version)
		os.Exit(0)
	}

	a := listAction
	if useAction == string(keepAction) {
		a = keepAction
	}

	sampleSize *= KB

	return a, fsLimit, verbose, roots, ignore, prefer, skipManual, dryRun, sampleSize

}

func main() {
	useAction, fsLimit, verbose, roots, ignore, prefer, skipManual, dryRun, sampleSize := getFlags()

	if len(roots) == 0 {
		roots = []string{"."}
	}

	fileSizes, err := getAllFileSizes(roots, ignore, verbose)
	if err != nil {
		fmt.Printf("filepath.Walk() returned an error: %v\n", err)
		return
	} else {
		fmt.Printf("Found %d unique file sizes\n", len(fileSizes))
	}

	sameSizeFiles, count := filterSameSizeFiles(fileSizes)
	if count > 0 {
		fmt.Printf("%d files need to be hashed:\n", count)
	} else {
		fmt.Printf("No files need to be hashed\n")
		return
	}

	sameHashFiles, count := filterSameHashFiles(sameSizeFiles, fsLimit, sampleSize, verbose)
	if count > 0 {
		fmt.Printf("%d files have duplicated hashes\n", count)
	} else {
		fmt.Printf("No files have duplicated hashes\n")
		return
	}

	execute(sameHashFiles, useAction, prefer, skipManual, dryRun)
}

// getAllFileSizes scans root directories recursively and returns the path of each file found
func getAllFileSizes(roots []string, ignore string, verbose bool) (map[int64][]string, error) {
	var (
		ignoreRegexp *regexp.Regexp
	)

	if ignore != "" {
		ignoreRegexp = regexp.MustCompile(ignore)
	}

	fileSizes := make(map[int64][]string)

	visit := func(path string, f os.FileInfo, err error) error {
		if f.IsDir() {
			return nil
		}

		if ignoreRegexp != nil && ignoreRegexp.MatchString(path) {
			return nil
		}

		p, err2 := filepath.EvalSymlinks(path)
		if err2 != nil {
			panic(err2)
		}
		if p != path {
			if verbose {
				log.Printf("symlink found: %s <-> %s\n", p, path)
			}
			return nil
		}

		if val, ok := fileSizes[f.Size()]; ok {
			fileSizes[f.Size()] = append(val, path)
		} else {
			fileSizes[f.Size()] = []string{path}
		}

		return nil
	}

	for _, root := range roots {
		err := filepath.Walk(root, visit)
		if err != nil {
			return nil, err
		}
	}

	for size, paths := range fileSizes {
		fileSizes[size] = uniqueStrings(paths)
	}

	return fileSizes, nil
}

// filterSameSizeFiles returns a list of file paths that have non-unique lengths
func filterSameSizeFiles(fileSizes map[int64][]string) (map[int64][]string, int) {
	sameSizeFiles := make(map[int64][]string)
	count := 0

	for size, files := range fileSizes {
		if len(files) <= 1 {
			continue
		}

		sameSizeFiles[size] = files
		count += len(files)
	}

	return sameSizeFiles, count
}

// filterSameHashFiles removes strings from a sameSizeFiles, and map all files that have a unique md5 hash
func filterSameHashFiles(sameSizeFiles map[int64][]string, fsLimit, sampleSize int, verbose bool) ([][]string, int) {
	var (
		sameHashFiles [][]string
		count, cur    int
	)

	for _, files := range sameSizeFiles {
		if verbose {
			fmt.Printf("Hashing files: %v\n", files)
		}

		uniqueHashes := getUniqueHashes(files, fsLimit, sampleSize, verbose)

		for _, paths := range uniqueHashes {
			if len(paths) > 1 {
				sameHashFiles = append(sameHashFiles, paths)
				count += len(paths)
			}
		}
		cur += 1
	}

	fmt.Println()

	return sameHashFiles, count
}

type md5ToHash struct {
	path string
	md5  string
	err  error
}

// hashWorker calculates the md5 hash value of a file and pushes it into a channel
func hashWorker(path string, md5s chan *md5ToHash, sampleSize int, verbose bool) {
	if verbose {
		fmt.Printf("About to read \"%s\"\n", path)
	}

	fi, err := os.Stat(path)
	if err != nil {
		log.Fatalf("can't stat file: %s, err: %v", path, err)
	}

	if fi.Size() < 1024 {
		sampleSize = int(fi.Size())
	}

	f, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}

	data := make([]byte, sampleSize)

	_, err = f.Read(data)
	if err != nil {
		log.Fatalf("error reading file: %s, err %v", path, err)
	}

	if err := f.Close(); err != nil {
		log.Fatalf("failed closing file: %s, err %v", path, err)
	}

	md5Hasher := md5.New()
	_, err = md5Hasher.Write(data)
	if err != nil {
		log.Fatalf("failed calculating hash for file: %s, err %v", path, err)
	}
	sum := md5Hasher.Sum(nil)

	if verbose {
		fmt.Printf("calculated md5 for file: %s\n", path)
	} else {
		fmt.Print(".")
	}

	md5s <- &md5ToHash{path, string(sum), nil}
}

// getUniqueHashes calculates the md5 hash of each file present in a map of sizes to paths of same size files
func getUniqueHashes(files []string, fsLimit, samleSize int, verbose bool) map[string][]string {
	md5s := make(chan *md5ToHash, fsLimit)

	for _, path := range files {
		go hashWorker(path, md5s, samleSize, verbose)
	}

	return getHashResults(md5s, len(files))
}

// collects worker results
func getHashResults(md5s chan *md5ToHash, max int) map[string][]string {
	uniqueHashes := make(map[string][]string)

	for i := 0; i < max; i++ {
		md5ToHash := <-md5s

		if md5ToHash.err != nil {
			fmt.Printf("\nhash returned an error: %v\n", md5ToHash.err)
			continue
		}

		if val, ok := uniqueHashes[md5ToHash.md5]; ok {
			uniqueHashes[md5ToHash.md5] = append(val, md5ToHash.path)
		} else {
			uniqueHashes[md5ToHash.md5] = []string{md5ToHash.path}
		}
	}

	return uniqueHashes
}

// execute deletes duplicates based on rules (prefer) and user input (unless skipManual is set)
func execute(sameSizeFiles [][]string, useAction action, prefer string, skipManual, dryRun bool) {
	var (
		preferRegexp *regexp.Regexp
	)

	if prefer != "" {
		preferRegexp = regexp.MustCompile(prefer)
	}

	fmt.Println()

	for i, files := range sameSizeFiles {
		fmt.Printf("The following files are the same (%d / %d):\n", i, len(sameSizeFiles))

		var answerMap = map[int]string{}
		for key, file := range files {
			if preferRegexp != nil && preferRegexp.MatchString(file) {
				fmt.Printf("[preferred] %s\n", file)
				continue
			}

			fmt.Printf("[%d] %s\n", key+1, file)

			answerMap[key] = file
		}

		if useAction == listAction {
			fmt.Printf("\n")
			continue
		}

		if len(answerMap) == len(files) && skipManual {
			fmt.Printf("Preferred file not found, deletion skipped.\n\n")
			continue
		}

		var deleteFiles []string
		if !skipManual && useAction == keepAction {
			deleteFiles = readKeep(answerMap, len(files))
		}

		if len(deleteFiles) == 0 {
			fmt.Printf("Deletion skipped.\n\n")
			continue
		}

		if len(deleteFiles) == len(files) {
			fmt.Printf("All files marked for deletion, therefore aborting!\n\n")
			continue
		}

		deleteOtherFiles(deleteFiles, dryRun)

		fmt.Printf("\n\n")
	}
}

// readKeep reads standard in to figure out which duplicates to keep
func readKeep(answerMap map[int]string, max int) []string {
	var (
		parsed []int
		res    []string
		ok     bool
	)

	fmt.Println("Which one of these should we keep? (eg: 1 2 3, 2-3)")

	scanner := bufio.NewScanner(os.Stdin)
	for !ok {
		scanner.Scan()
		s := scanner.Text()
		if s == "" {
			break
		}

		parsed, ok = parseRead(s, max)
		if !ok {
			fmt.Print("again: ")
			continue
		}

		if !allParsedFound(parsed, answerMap) {
			ok = false
			continue
		}
	}

	for _, v := range parsed {
		_, ok := answerMap[v-1]
		if !ok {
			fmt.Print("again: ")
			continue
		}

		delete(answerMap, v-1)
	}

	for _, f := range answerMap {
		res = append(res, f)
	}

	return res
}

// allParsedFound returns true if all numbers read from the standard in our in the answerMap
func allParsedFound(parsed []int, answerMap map[int]string) bool {
	for _, v := range parsed {
		_, ok := answerMap[v-1]
		if !ok {
			return false
		}
	}

	return true
}

// parseRead parses a line read from standard in as numbers for files to keep
func parseRead(s string, max int) ([]int, bool) {
	if s == "" {
		return nil, false
	}

	elements := strings.Split(s, " ")

	if len(elements) < 1 {
		return nil, false
	}

	var res []int
	for _, elem := range elements {
		parts, ok := parseElem(elem, max)
		if !ok {
			return nil, false
		}

		res = append(res, parts...)
	}

	if len(res) == 0 {
		return res, true
	}

	res = uniqueInts(res)

	return res, true
}

// parseElem will parse one part of a line read from standard in (part is separated by spaces)
func parseElem(elem string, max int) ([]int, bool) {
	parts := strings.Split(elem, "-")

	if len(parts) > 2 {
		return nil, false
	}

	if len(parts) == 2 {
		return generateRange(parts[0], parts[1], max)
	}

	return generateRange(parts[0], parts[0], max)
}

// generateRange
func generateRange(from, to string, max int) ([]int, bool) {
	if from == "" || to == "" {
		return nil, false
	}

	ar, err := strconv.ParseInt(from, 10, 64)
	if err != nil {
		return nil, false
	}
	a := int(ar)

	br, err := strconv.ParseInt(to, 10, 64)
	if err != nil {
		return nil, false
	}
	b := int(br)

	if a > b || a < 1 || b > max {
		return nil, false
	}

	var res []int
	for i := a; i <= b; i++ {
		res = append(res, i)
	}

	return res, true
}

// deleteOtherFiles deletes a list of files, unless dryRun is set
func deleteOtherFiles(deleteFiles []string, dryRun bool) {
	for _, file := range deleteFiles {
		if dryRun {
			fmt.Printf("Removing: %s (skipped)\n", file)
			continue
		}

		fmt.Printf("Removing: %s\n", file)

		err := os.Remove(file)
		if err != nil {
			fmt.Printf("%v\n", err)
		} else {
			fmt.Println("done.")
		}
	}
}

// uniqueInts returns unique integers from a list of integers
func uniqueInts(ints []int) []int {
	all := map[int]int{}
	for _, val := range ints {
		all[val] = val
	}

	var res []int
	for val := range all {
		res = append(res, val)
	}

	sort.Ints(res)

	return res
}

// uniqueStrings returns unique strings from a list of strings
func uniqueStrings(arr []string) []string {
	all := map[string]string{}
	for _, val := range arr {
		all[val] = val
	}

	var res []string
	for val := range all {
		if val == "" {
			continue
		}

		res = append(res, val)
	}

	sort.Strings(res)

	return res
}
