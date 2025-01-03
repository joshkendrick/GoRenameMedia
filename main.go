// Author: Josh Kendrick
// Version: v0.4.0
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
	renamedCount, issues := fileProcessor(filepaths)

	// loop through and print any issues
	for _, issue := range issues {
		if issue.err == nil {
			break
		}
		fileNameRel := filepath.Base(issue.path)
		log.Printf("!!ERROR!! -- %v -> %v", fileNameRel, issue.err)
	}

	// print results
	log.Printf("renamed: %d || issues %d", renamedCount, len(issues))
}

func fileProcessor(filepaths <-chan string) (renamed int, issues []FileError) {
	// create the exifReader
	// this isnt flexible, as is will only work on windows with exiftool.exe in same location as execution
	exifReader, err := exiftool.NewExiftool(exiftool.SetExiftoolBinaryPath(EXIFTOOL_PATH))
	if err != nil {
		log.Printf("!!ERROR!! -- %v", err)
		return
	}

	var issue FileError
	var count int
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
		// check it exists
		if err != nil {
			issue = FileError{fileNameRel, err}
			issues = append(issues, issue)
			continue
		}
		// and is a supported type
		fileExt, err := checkFileTypeSupport(fileType)
		if err != nil {
			issue = FileError{fileNameRel, err}
			issues = append(issues, issue)
			continue
		}

		// use file info to generate the new fileName for renaming
		newFileName, newFilePath, err := generateNewName(filenameAbs, fileInfo, fileExt)
		if err != nil {
			issue = FileError{fileNameRel, err}
			issues = append(issues, issue)
			continue
		}

		if fileNameRel != newFileName {
			log.Printf("renaming %v to %v", fileNameRel, newFileName)
			// rename the file
			err = os.Rename(filenameAbs, newFilePath)
			if err != nil {
				issue = FileError{fileNameRel, err}
				issues = append(issues, issue)
				continue
			} else {
				renamed++
			}
		}
		count++
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

func checkFileTypeSupport(fileType string) (fileExt string, err error) {
	if fileType == JPEG {
		fileExt = JPEG_EXT
	} else if fileType == HEIC {
		fileExt = HEIC_EXT
	} else if fileType == PNG {
		fileExt = PNG_EXT
	} else if fileType == MP4 {
		fileExt = MP4_EXT
	} else if fileType == MOV {
		fileExt = MOV_EXT
	} else {
		err = fmt.Errorf("!!INFO!! -- not a supported type %v", fileType)
	}

	return fileExt, err
}

// Date/Time formats
const JPEG_FORMAT = "2006:01:02 15:04:05"
const HEIC_FORMAT = JPEG_FORMAT + ".000-07:00"
const MOV_FORMAT = JPEG_FORMAT + "-07:00"
const RENAME_FORMAT = "2006-01-02_150405"

func generateNewName(filenameAbs string, fileInfo exiftool.FileMetadata, fileExt string) (newFileName string, newFilePath string, err error) {
	// attempt to read the date/time from a range of fields
	dateTime, _ := fileInfo.GetString("DateTimeOriginal") // typical for JPEG, PNG, and MP4
	if dateTime == "" {
		dateTime, _ = fileInfo.GetString("CreationDate") // MOV
	}
	if dateTime == "" {
		dateTime, _ = fileInfo.GetString("SubSecDateTimeOriginal") // HEIC
	}
	if dateTime == "" {
		dateTime, _ = fileInfo.GetString("ContentCreateDate") // sometimes MOV / MP4
	}
	if dateTime == "" {
		dateTime, _ = fileInfo.GetString("CreateDate") // sometimes JPEG
	}
	if dateTime == "" { // not able to get a date/time, error out
		err = errors.New("!!INFO!! -- not able to extract a date/time")
		return newFileName, newFilePath, err
	}

	// attempt to parse
	dateTimeParsed, err := time.Parse(JPEG_FORMAT, dateTime)
	if err != nil {
		dateTimeParsed, err = time.Parse(HEIC_FORMAT, dateTime)
	}
	if err != nil {
		dateTimeParsed, err = time.Parse(MOV_FORMAT, dateTime)
	}
	if err != nil {
		err = fmt.Errorf("!!INFO!! -- not able to parse to a date/time %v", dateTimeParsed)
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

		// if testErr is nil, a file with that name was found, so increment and try again
		// but only if the generated name is different from existing
		if _, testErr := os.Stat(testFilePath); testErr == nil && testFilePath != filenameAbs {
			i++
		} else {
			newFileName = testFilename
			newFilePath = testFilePath
			break
		}
	}

	return newFileName, newFilePath, err
}
