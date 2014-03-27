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
		fmt.Printf("no files need to be hashed\n", len(same_size_files)*2)
		return
	}

	same_hash_files, count := filterSameHashFiles(same_size_files)
	if count > 0 {
		fmt.Printf("%d files have duplicated hashes\n", count)
	} else {
		fmt.Printf("no files has duplicated hashes\n", len(same_size_files)*2)
		return
	}

	cleanUp(same_hash_files)
}

func getRoot() string {
	flag.Parse()
	return flag.Arg(0)
}

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

func hash(path string) (string, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}

	h := md5.New()

	h.Write(data)

	return string(h.Sum(nil)), nil
}

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

func getUniqueHashes(files []string) map[string][]string {
	unique_hashes := make(map[string][]string)

	for _, path := range files {
		md5, err := hash(path)

		if err != nil {
			fmt.Printf("hash returned an error: %v\n", err)
			continue
		}

		if val, ok := unique_hashes[md5]; ok {
			unique_hashes[md5] = append(val, path)
		} else {
			unique_hashes[md5] = []string{path}
		}

		globalCount += 1

		fmt.Printf("%d ", globalCount)

		if globalCount%10 == 0 {
			fmt.Println()
		}
	}

	return unique_hashes
}

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
