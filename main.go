// Copyright 2014 DevMonk. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Dblfinder provides a command-line tool for finding duplicated files
*/
package main

import (
	"crypto/md5"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

func main() {
	root := getRoot()

	filesizes, err := getAllFilesizes(root)
	if err != nil {
		fmt.Printf("filepath.Walk() returned an error: %v\n", err)
		return
	} else {
		fmt.Printf("visited at least %d files\n", len(filesizes))
	}

	same_size_files, count := filterSameSizeFiles(filesizes)
	if count > 0 {
		fmt.Printf("%d files need to be hashed\n", count)
	} else {
		fmt.Printf("no files need to be hashed\n")
		return
	}

	same_hash_files, count2 := filterSameHashFiles(same_size_files)
	if count2 > 0 {
		fmt.Printf("%d files have duplicated hashes\n", count2)
	} else {
		fmt.Printf("no files has duplicated hashes\n")
		return
	}

	cleanUp(same_hash_files)
}

// getRoot returns the first argument provided
func getRoot() string {
	flag.Parse()
	return flag.Arg(0)
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
	same_size_files := make(map[int64][]string)
	count := 0

	for size, files := range filesizes {
		if len(files) > 1 {
			same_size_files[size] = files
			count += len(files)
		}
	}

	return same_size_files, count
}

// filterSameHashFiles removes strings from a same_size_files map all files that have a unique md5 hash
func filterSameHashFiles(same_size_files map[int64][]string) ([][]string, int) {
	same_hash_files := [][]string{}
	count := 0

	for _, files := range same_size_files {
		unique_hashes := getUniqueHashes(files)

		for _, paths := range unique_hashes {
			if len(paths) > 1 {
				same_hash_files = append(same_hash_files, paths)
				count += len(paths)
			}
		}
	}

	fmt.Println()

	return same_hash_files, count
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

// hash calculates the md5 hash value of a file and puts it into a channel
func hashWorker(path string, md5s chan *md5ToHash) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		md5s <- &md5ToHash{path, "", err}
		return
	}

	h := md5.New()

	h.Write(data)

	md5s <- &md5ToHash{path, string(h.Sum(nil)), nil}
}

// getUniqueHashes calculates the md5 hash of each file present in a map of sizes to paths of same size files
func getUniqueHashes(files []string) map[string][]string {
	md5s := make(chan *md5ToHash)

	for _, path := range files {
		go hashWorker(path, md5s)
	}

	return getHashResults(md5s, len(files))
}

// collects worker results
func getHashResults(md5s chan *md5ToHash, max int) map[string][]string {
	unique_hashes := make(map[string][]string)

	for i := 0; i < max; i++ {
		md5_to_hash := <-md5s

		count := getCount()
		fmt.Printf("%d ", count)

		if count%10 == 0 {
			fmt.Println()
		}

		if md5_to_hash.err != nil {
			fmt.Printf("\nhash returned an error: %v\n", md5_to_hash.err)
			continue
		}

		if val, ok := unique_hashes[md5_to_hash.md5]; ok {
			unique_hashes[md5_to_hash.md5] = append(val, md5_to_hash.path)
		} else {
			unique_hashes[md5_to_hash.md5] = []string{md5_to_hash.path}
		}
	}

	return unique_hashes
}

// cleanUp deletes all, but one instance of the same file
// number of kept file is read from standard input (count starts from 1)
// number zero returned will skip file deletion
// os part is done in deleteAllFilesButI
func cleanUp(same_hash_files [][]string) {
	for _, files := range same_hash_files {
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
	del_files := append(files[:i-1], files[i:]...)

	for _, file := range del_files {
		fmt.Printf("Removing: %s\n", file)

		err := os.Remove(file)
		if err != nil {
			fmt.Printf("%v\n", err)
		} else {
			fmt.Println("done.")
		}
	}
}
