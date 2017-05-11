/*
Dblfinder provides a command-line tool for finding duplicated files.
When duplicates are found, it can provide an option to delete one of the them.
*/
package main

import (
	"crypto/md5"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"

	docopt "github.com/docopt/docopt-go"
)

const name = "dblfinder"
const version = "0.2.0"
const usage = `
Dblfinder provides a command-line tool for finding duplicated files.
When duplicates are found, it can provide an option to delete one of the them.

Usage:
  dblfinder -h | --help
  dblfinder -v | --version
  dblfinder [--fix] [--limit=<n>] [--verbose] <root>

Options:
  -h --help     display help
  -v --version  display version number
  --fix         try to fix issues, not only list them
  --limit=<n>   limit the maximum number of duplicates to fix [default: 0]
  --verbose     provide verbose output
`

func main() {
	fix, limit, verbose, root, err := getFlags()
	if root == "" {
		fmt.Printf("No root is provided", err)
		return
	}

	filesizes, err := getAllFilesizes(root)
	if err != nil {
		fmt.Printf("filepath.Walk() returned an error: %v\n", err)
		return
	} else {
		fmt.Printf("Visited at least %d files\n", len(filesizes))
	}

	sameSizeFiles, count := filterSameSizeFiles(filesizes)
	if count > 0 {
		fmt.Printf("%d files were be hashed\n", count)
	} else {
		fmt.Printf("No files were be hashed\n")
		return
	}

	sameHashFiles, count := filterSameHashFiles(sameSizeFiles, limit, verbose)
	if count > 0 {
		fmt.Printf("%d files have duplicated hashes\n", count)
	} else {
		fmt.Printf("No files have duplicated hashes\n")
		return
	}

	if fix {
		cleanUp(sameHashFiles, limit)
	} else {
		listAll(sameHashFiles, limit)
	}
}

func getFlags() (bool, int, bool, string, error) {
	var (
		fix      bool
		limit    int
		verbose  bool
		root     string
		rawLimit string
		limit64  int64
	)

	arguments, err := docopt.Parse(usage, nil, true, fmt.Sprintf("%s %s", name, version), false)
	if err != nil {
		return fix, limit, verbose, root, err
	}

	if arguments["fix"] != nil {
		fix = true
	}
	if arguments["verbose"] != nil {
		verbose = true
	}
	if arguments["limit"] != nil {
		rawLimit = arguments["limit"].(string)
	}
	root = arguments["<root>"].(string)

	if rawLimit != "" {
		limit64, err = strconv.ParseInt(rawLimit, 10, 64)

		limit = int(limit64)
		if limit < 0 {
			limit = 0
		}
	}

	return fix, limit, verbose, root, nil
}

// getAllFilesizes scans the root directory recursively and returns the path of each file found
func getAllFilesizes(root string) (map[int64][]string, error) {
	filesizes := make(map[int64][]string)

	visit := func(path string, f os.FileInfo, err error) error {
		if f.IsDir() {
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

// filterSameHashFiles removes strings from a sameSizeFiles map all files that have a unique md5 hash
func filterSameHashFiles(sameSizeFiles map[int64][]string, limit int, verbose bool) ([][]string, int) {
	sameHashFiles := [][]string{}
	count := 0
	cur := 0

	for _, files := range sameSizeFiles {
		if limit > 0 && cur >= limit {
			fmt.Printf("\nHashing limit is reached.\n")
			break
		}

		uniqueHashes := getUniqueHashes(files, verbose)

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

var globalCount int

// getCount returns a unique, incremented value
func getCount() int {
	globalCount += 1
	return globalCount
}

type md5ToHash struct {
	path string
	md5  string
	err  error
}

// hashWorker calculates the md5 hash value of a file and pushes it into a channel
func hashWorker(path string, md5s chan *md5ToHash, verbose bool) {
	if verbose {
		fmt.Printf("About to read \"%s\"\n", path)
	}

	data, err := ioutil.ReadFile(path)
	if err != nil {
		if verbose {
			fmt.Printf("Reading data for \"%s\" failed.\n", path)
		}
		md5s <- &md5ToHash{path, "", err}
		return
	}

	h := md5.New()

	h.Write(data)

	sum := h.Sum(nil)

	if verbose {
		fmt.Printf("Calculated md5 of \"%s\".\n", path)
	}

	md5s <- &md5ToHash{path, string(sum), nil}
}

// getUniqueHashes calculates the md5 hash of each file present in a map of sizes to paths of same size files
func getUniqueHashes(files []string, verbose bool) map[string][]string {
	md5s := make(chan *md5ToHash)

	for _, path := range files {
		go hashWorker(path, md5s, verbose)
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
// os part is done in deleteAllFilesButI
func cleanUp(sameSizeFiles [][]string, limit int) {
	for key, files := range sameSizeFiles {
		if limit > 0 && key >= limit {
			fmt.Println("Cleanup limit is reached.")
			break
		}

		fmt.Println("The following files are the same:")

		for key, file := range files {
			fmt.Printf("[%d] %s\n", key+1, file)
		}

		i := readInt(len(files))

		if i == 0 {
			fmt.Printf("Deletion skipped.\n\n")
		} else {
			fmt.Printf("Deleting all, but `%s`.\n", files[i-1])

			deleteAllFilesButI(files, i)

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

// deleteAllFilesButI deletes a list of files, except for the i.-th file, counting from 1
func deleteAllFilesButI(files []string, i int) {
	delFiles := append(files[:i-1], files[i:]...)

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
// os part is done in deleteAllFilesButI
func listAll(sameSizeFiles [][]string, limit int) {
	for key, files := range sameSizeFiles {
		if limit > 0 && key >= limit {
			fmt.Println("Listing limit is reached.")
			break
		}

		fmt.Println("The following files are the same:")

		for key, file := range files {
			fmt.Printf("[%d] %s\n", key+1, file)
		}

		fmt.Println()
	}
}
