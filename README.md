Dblfinder
=========

Dblfinder (pronounce it as double finder) provides a command-line tool for finding duplicated files.
When duplicates are found, it can provide an option to delete one of the them.

How it works:
1. It scans the directory structure under `root` and groups them by filesize.
2. It loops through the created group sizes and hashes all files in a group with the same size.
3. When it finds files with the same non-zero filesize and the same hash, it identifies them as being the same.
4. When run with the `fix` option, it will offer to keep only one of the same files, otherwise it just lists duplicates

Usage:
  *# Display help*
  dblfinder -h | --help
  *# Display version number*
  dblfinder -v | --version
  *# Find duplicates recursively in the directory provided*
  dblfinder [--fix] [--limit=<n>] [--verbose] <root>

Options:
  -h --help     display help
  -v --version  display version number
  --fix         try to fix issues, not only list them
  --limit=<n>   limit the maximum number of duplicates to fix [default: 0]
  --verbose     provide verbose output

