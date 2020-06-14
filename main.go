/*
Dblfinder provides a command-line tool for finding duplicated files.
When duplicates are found, it can provide an option to delete one of the them.
*/
package main

import (
	"crypto/md5"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sync"
)

const version = "0.4.3"

func getFlags() (bool, int, bool, string, string, bool) {
	var (
		showHelp, showVersion, verbose, fix, skipManual bool
		fsLimit                                         int
		prefer, root                                    string
	)

	flag.BoolVar(&showHelp, "help", false, "display help")
	flag.BoolVar(&showVersion, "version", false, "display the version number")
	flag.BoolVar(&verbose, "verbose", false, "provide verbose output")
	flag.IntVar(&fsLimit, "fs-limit", 0, "limit the maximum number open files")
	flag.BoolVar(&fix, "fix", false, "try to fix issues, not only list them")
	flag.StringVar(&prefer, "prefer", "", "limit the maximum number of duplicates to fix")
	flag.BoolVar(&skipManual, "skip-manual", false, "skip decisions if prefer did not find anything")

	flag.Parse()

	root = flag.Arg(0)

	if showHelp {
		flag.PrintDefaults()
		os.Exit(0)
	}

	if showVersion {
		fmt.Println(version)
		os.Exit(0)
	}

	return fix, fsLimit, verbose, root, prefer, skipManual
}

func main() {
	fix, fsLimit, verbose, root, prefer, skipManual := getFlags()

	if root == "" {
		root = "."
	}

	filesizes, err := getAllFilesizes(root)
	if err != nil {
		fmt.Printf("filepath.Walk() returned an error: %v\n", err)
		return
	} else {
		fmt.Printf("Found %d unique filenames\n", len(filesizes))
	}

	sameSizeFiles, count := filterSameSizeFiles(filesizes)
	if count > 0 {
		fmt.Printf("%d files need to be hashed:\n", count)
	} else {
		fmt.Printf("No files need to be hashed\n")
		return
	}

	sameHashFiles, count := filterSameHashFiles(sameSizeFiles, fsLimit, verbose)
	if count > 0 {
		fmt.Printf("%d files have duplicated hashes\n", count)
	} else {
		fmt.Printf("No files have duplicated hashes\n")
		return
	}

	if fix {
		cleanUp(sameHashFiles, prefer, skipManual)
	} else {
		listAll(sameHashFiles)
	}
}

// getAllFilesizes scans the root directory recursively and returns the path of each file found
func getAllFilesizes(root string) (map[int64][]string, error) {
	filesizes := make(map[int64][]string)

	visit := func(path string, f os.FileInfo, err error) error {
		if f.IsDir() {
			return nil
		}

		p, err2 := filepath.EvalSymlinks(path)
		if err2 != nil {
			panic(err2)
		}
		if p != path {
			return nil
		}

		if val, ok := filesizes[f.Size()]; ok {
			filesizes[f.Size()] = append(val, path)
		} else {
			filesizes[f.Size()] = []string{path}
		}

		return nil
	}

	err := filepath.Walk(root, visit)

	return filesizes, err
}

// filterSameSizeFiles returns a list of filepaths that have non-unique length
func filterSameSizeFiles(filesizes map[int64][]string) (map[int64][]string, int) {
	sameSizeFiles := make(map[int64][]string)
	count := 0

	for size, files := range filesizes {
		if len(files) > 1 {
			sameSizeFiles[size] = files
			count += len(files)
		}
	}

	return sameSizeFiles, count
}

// filterSameHashFiles removes strings from a sameSizeFiles, and map all files that have a unique md5 hash
func filterSameHashFiles(sameSizeFiles map[int64][]string, fsLimit int, verbose bool) ([][]string, int) {
	sameHashFiles := [][]string{}
	count := 0
	cur := 0

	for _, files := range sameSizeFiles {
		if verbose {
			fmt.Printf("Hashing files: %v\n", files)
		}

		uniqueHashes := getUniqueHashes(files, fsLimit, verbose)

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
func hashWorker(path string, md5s chan *md5ToHash, wg *sync.WaitGroup, verbose bool) {
	wg.Add(1)
	defer wg.Done()

	if verbose {
		fmt.Printf("About to read \"%s\"\n", path)
	}

	f, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}


	data := make([]byte, 1024)

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
func getUniqueHashes(files []string, fsLimit int, verbose bool) map[string][]string {
	var wg sync.WaitGroup
	md5s := make(chan *md5ToHash)

	for i, path := range files {
		go hashWorker(path, md5s, &wg, verbose)
		if fsLimit > 0 && (i % fsLimit == 0) {
			if verbose {
				fmt.Printf("waiting for waiting group...\n")
			}
			wg.Wait()
		}
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

// cleanUp deletes all, but one instance of the same file
// number of kept file is read from standard input (count starts from 1)
// number zero returned will skip file deletion
// os part is done in deleteOtherFiles
func cleanUp(sameSizeFiles [][]string, prefer string, skipManual bool) {
	var (
		preferRegexp *regexp.Regexp
		keep         int
	)

	if prefer != "" {
		preferRegexp = regexp.MustCompile(prefer)
	}

	for _, files := range sameSizeFiles {
		fmt.Println("The following files are the same:")

		keep = 0
		for key, file := range files {
			fmt.Printf("[%d] %s\n", key+1, file)

			if preferRegexp == nil || keep < 0 || !preferRegexp.MatchString(file) {
				continue
			}

			// We found more than one preferred file here...
			if keep > 0 {
				keep = -1
				continue
			}

			keep = key + 1
		}

		if keep < 1 && skipManual {
			fmt.Printf("Preferred file not found, deletion skipped.\n\n")
			continue
		}

		for keep < 1 || keep > len(files) {
			keep = readInt(len(files))
		}

		if keep == 0 {
			fmt.Printf("Deletion skipped.\n\n")
		} else if keep > 0 {
			fmt.Printf("Deleting all, but `%s`.\n", files[keep-1])

			deleteOtherFiles(files, keep)

			fmt.Printf("\n\n")
		}
	}
}

// readInt reads an integer from standard input, minimum value is 0, maximum is given as parameter
func readInt(max int) int {
	var i int

	for i < 1 || i > max {
		fmt.Println("Which one of these should we keep? (O for keeping all)")
		_, err := fmt.Scanf("%d", &i)

		if err != nil {
			i = 0
			continue
		}

		if i == 0 {
			return 0
		}
	}

	return i
}

// deleteOtherFiles deletes a list of files, except for the i.-th file, counting from 1
func deleteOtherFiles(files []string, keep int) {
	delFiles := append(files[:keep-1], files[keep:]...)

	for _, file := range delFiles {
		fmt.Printf("Removing: %s\n", file)

		err := os.Remove(file)
		if err != nil {
			fmt.Printf("%v\n", err)
		} else {
			fmt.Println("done.")
		}
	}
}

// cleanUp deletes all, but one instance of the same file
// number of kept file is read from standard input (count starts from 1)
// number zero returned will skip file deletion
// os part is done in deleteOtherFiles
func listAll(sameSizeFiles [][]string) {
	for _, files := range sameSizeFiles {
		fmt.Println("The following files are likely the same:")

		for key, file := range files {
			fmt.Printf("[%d] %s\n", key+1, file)
		}

		fmt.Println()
	}
}
