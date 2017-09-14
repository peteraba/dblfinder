Dblfinder
=========

Dblfinder provides a command-line tool for finding duplicated files.
When duplicates are found, it can provide an option to delete one of the them.

How it works:
1. It scans the directory structure under `root` and groups them by filesize.
2. It loops through each group and tries to decide if they are the same byhashing the first 1KB of each file and collects group of files with the same size and same first 1KB of data.
3. At this point it can do different things, depending on the options:
  1. It can simply list the files which seem to be the same
  2. It can offer deleting files by group
  3. It can check if there's only one file matching a regular expression (prefer), and keep only that automatically.
  4. If skip-manual is provided, groups without a preferred file found will be skipped.


```
Usage:
  dblfinder --help
  dblfinder --version
  dblfinder [--fix] [--limit=<n>] [--verbose] <root>

Options:
  --help         display help
  --version      display version number
  --verbose      provide verbose output
  --fix          try to fix issues, not only list them
  --prefer=<s>   prefer path if it matches regexp defined here
  --skip-manual  skip decisions if prefer did not find anything
  --limit=<n>    limit the maximum number of duplicates to fix [default: 0]
```
