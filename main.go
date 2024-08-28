// Author: Josh Kendrick
// Version: v0.2.0
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
	go func() {
		filepath.Walk(directory, func(path string, f os.FileInfo, err error) error {
			if !f.IsDir() {
				filepaths <- path
			}

			return nil
		})
		// done adding files
		close(filepaths)
	}()

	// start the processor
	completedCount, issues := fileProcessor(filepaths)

	// loop through and print any issues
	for _, issue := range issues {
		if issue.err == nil {
			break
		}
		fileNameRel := filepath.Base(issue.path)
		log.Printf("!!ERROR!! -- %v -> %v", fileNameRel, issue.err)
	}

	// print results
	log.Printf("renamed: %d || issues %d", completedCount, len(issues))
}

func fileProcessor(filepaths <-chan string) (count int, issues []FileError) {
	// create the exifReader
	// this isnt flexible, as is will only work on windows with exiftool.exe in same location as execution
	exifReader, err := exiftool.NewExiftool(exiftool.SetExiftoolBinaryPath(EXIFTOOL_PATH))
	if err != nil {
		log.Printf("!!ERROR!! -- %v", err)
		return
	}

	var issue FileError
	// get a filepath
	for {
		filenameAbs, more := <-filepaths
		// if no more, then stop + return
		if !more {
			log.Printf("consumed %d files", count)
			return
		}

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

		// use file info to generate the new fileName for renaming
		newFileName, newFilePath, err := generateNewName(filenameAbs, fileInfo, fileType)
		if err != nil {
			issue = FileError{fileNameRel, err}
			issues = append(issues, issue)
			continue
		}

		log.Printf("renaming %v to %v", fileNameRel, newFileName)
		// rename the file
		err = os.Rename(filenameAbs, newFilePath)
		if err != nil {
			issue = FileError{fileNameRel, err}
			issues = append(issues, issue)
			continue
		} else {
			count++
		}
	}
}

// Filetypes
const JPEG = "JPEG"
const JPEG_EXT = ".jpg"
const HEIC = "HEIC"
const HEIC_EXT = ".heic"
const PNG = "PNG"
const PNG_EXT = ".png"
const MP4 = "MP4"
const MP4_EXT = ".mp4"
const MOV = "MOV"
const MOV_EXT = ".mov"

// Date/Time formats
const JPEG_FORMAT = "2006:01:02 15:04:05"
const HEIC_FORMAT = JPEG_FORMAT + ".000-07:00"
const MOV_FORMAT = JPEG_FORMAT + "-07:00"
const RENAME_FORMAT = "2006-01-02_150405"

func generateNewName(filenameAbs string, fileInfo exiftool.FileMetadata, fileType string) (newFileName string, newFilePath string, err error) {
	// parse and get renaming values
	var dateTimeParsed time.Time
	var fileExt string
	if fileType == JPEG {
		dateTime, _ := fileInfo.GetString("DateTimeOriginal")
		fileExt = JPEG_EXT
		dateTimeParsed, err = time.Parse(JPEG_FORMAT, dateTime)
	} else if fileType == HEIC {
		dateTime, _ := fileInfo.GetString("SubSecDateTimeOriginal")
		fileExt = HEIC_EXT
		dateTimeParsed, err = time.Parse(HEIC_FORMAT, dateTime)
	} else if fileType == PNG {
		dateTime, _ := fileInfo.GetString("DateTimeOriginal")
		fileExt = PNG_EXT
		dateTimeParsed, err = time.Parse(JPEG_FORMAT, dateTime)
	} else if fileType == MP4 {
		dateTime, _ := fileInfo.GetString("DateTimeOriginal")
		fileExt = MP4_EXT
		dateTimeParsed, err = time.Parse(MOV_FORMAT, dateTime)
	} else if fileType == MOV {
		dateTime, _ := fileInfo.GetString("CreationDate")
		fileExt = MOV_EXT
		dateTimeParsed, err = time.Parse(MOV_FORMAT, dateTime)
	} else {
		errorText := fmt.Sprintf("!!INFO!! -- not a supported type %v", fileType)
		err = errors.New(errorText)
	}

	// if there was an error, return before renaming
	if err != nil {
		return newFileName, newFilePath, err
	}

	// find a new filename, create the new filepath
	newFileName = dateTimeParsed.Format(RENAME_FORMAT)
	fileDir := filepath.Dir(filenameAbs)
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

	return newFileName, newFilePath, err
}
