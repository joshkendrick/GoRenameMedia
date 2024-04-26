// Author: Josh Kendrick
// Version: v0.0.1
// License: do whatever you want with this code

package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/barasher/go-exiftool"
)

const EXIFTOOL_PATH = "./exiftool.exe"

// Filetypes
const JPEG = "JPEG"
const JPEG_EXT = ".jpg"
const MOV = "MOV"
const MOV_EXT = ".mov"

// Date/Time formats
const JPEG_FORMAT = "2006:01:02 15:04:05"
const MOV_FORMAT = JPEG_FORMAT + "-07:00"
const RENAME_FORMAT = "2006-01-02_150405"

// struct to hold pieces of a db statement
type FileError struct {
	path string
	err  error
}

func main() {
	if len(os.Args) < 2 {
		log.Fatal("must specify a directory")
	}
	// parent dir
	directory := os.Args[1]

	// produce the files to the channel for the processor
	filepaths := make(chan string, 100)
	producedCount := 0
	go func() {
		filepath.Walk(directory, func(path string, f os.FileInfo, err error) error {
			if !f.IsDir() {
				filepaths <- path
				producedCount++
			}

			return nil
		})
		// done adding files
		close(filepaths)
	}()

	// reporting channel
	processorDone := make(chan int)
	// slice to hold issues
	issues := make([]FileError, 10)
	// start the processor
	go fileProcessor(filepaths, processorDone, issues)

	// wait for processor to finish
	consumedCount := <-processorDone
	close(processorDone)

	// loop through and print any issues
	for _, issue := range issues {
		if issue.err == nil {
			break
		}
		fileNameRel := filepath.Base(issue.path)
		log.Printf("!!ERROR!! -- %v: %v", fileNameRel, issue.err)
	}

	// print results
	log.Printf("produced: %d || consumed %d", producedCount, consumedCount)
}

func fileProcessor(filepaths <-chan string, done chan<- int, issues []FileError) {
	var issue FileError
	count := 0

	// create the exifReader
	// this isnt flexible, as is will only work on windows with exiftool.exe in same location as execution
	exifReader, err := exiftool.NewExiftool(exiftool.SetExiftoolBinaryPath(EXIFTOOL_PATH))
	if err != nil {
		log.Printf("!!ERROR!! -- %v", err)
		return
	}

	// get a filepath
	for {
		filenameAbs, more := <-filepaths
		if !more {
			log.Printf("consumed %d files", count)
			done <- count
			return
		}
		count++

		// get the filename
		fileNameRel := filepath.Base(filenameAbs)

		// get the metadata of the file
		// there should only be one FileInfo since we call for one filepath
		fileInfo := exifReader.ExtractMetadata(filenameAbs)[0]
		if fileInfo.Err != nil {
			issue = FileError{fileNameRel, fileInfo.Err}
			issues = append(issues, issue)
			continue
		}

		// get the file type
		fileType, err := fileInfo.GetString("FileType")
		if err != nil {
			issue = FileError{fileNameRel, err}
			issues = append(issues, issue)
			continue
		}

		// parse and get renaming values
		var dateTimeParsed time.Time
		var fileExt string
		if fileType == JPEG {
			dateTime, _ := fileInfo.GetString("DateTimeOriginal")
			fileExt = JPEG_EXT
			dateTimeParsed, err = time.Parse(JPEG_FORMAT, dateTime)
		} else if fileType == MOV {
			dateTime, _ := fileInfo.GetString("CreationDate")
			fileExt = MOV_EXT
			dateTimeParsed, err = time.Parse(MOV_FORMAT, dateTime)
		} else {
			errorText := fmt.Sprintf("!!INFO!! -- not a supported type %v", fileType)
			err = errors.New(errorText)
		}
		if err != nil {
			issue = FileError{fileNameRel, err}
			issues = append(issues, issue)
			continue
		}

		// find a new filename, create the new filepath
		newFileName := dateTimeParsed.Format(RENAME_FORMAT) // + fileExt
		fileDir := filepath.Dir(filenameAbs)
		var newFilePath string
		i := 1
		for {
			index := fmt.Sprintf("%02d", i)
			testFilename := newFileName + "_" + index + fileExt
			testFilePath := filepath.Join(fileDir, testFilename)
			if _, testErr := os.Stat(testFilePath); testErr == nil {
				i++
			} else {
				newFileName = testFilename
				newFilePath = testFilePath
				break
			}
		}

		log.Printf("renaming %v to %v", fileNameRel, newFileName)
		// rename the file
		err = os.Rename(filenameAbs, newFilePath)
		if err != nil {
			issue = FileError{fileNameRel, err}
			issues = append(issues, issue)
			continue
		}
	}
}
