package main

/*
 * Module Dependencies
 */

import (
	"crypto/md5"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mozzzzy/arguments"
	"github.com/mozzzzy/arguments/option"
	"github.com/mozzzzy/clitool"
)

/*
 * Types
 */

/*
 * Constants and Package Scope Variables
 */

const (
	BEAD_BUF_SIZE int = 1024
)

var (
	artifactMode  os.FileMode = 0644
	tmpFileDir    string      = "/tmp"
	tmpFilePrefix string      = "join_file."
)

/*
 * Functions
 */

func getPartialFilePaths(partialFileDir, partialFilePrefix string) ([]string, error) {
	var partialFiles []string
	// Get all files in partial file directory
	files, readDirErr := ioutil.ReadDir(partialFileDir)
	if readDirErr != nil {
		return nil, readDirErr
	}
	for _, file := range files {
		if !strings.HasPrefix(file.Name(), partialFilePrefix) {
			continue
		}
		if file.IsDir() {
			continue
		}
		partialFiles = append(partialFiles, partialFileDir+"/"+file.Name())
	}
	return partialFiles, nil
}

func printErrorAndWaitEsc(msg string) {
	clitool.Error(msg)
	clitool.Message("Please esc key to exit.")
	clitool.WaitEsc()
	return
}

func readFile(filePath string) ([]byte, int, error) {
	// Open file
	fp, openErr := os.Open(filePath)
	if openErr != nil {
		return nil, 0, openErr
	}

	// Create buffer
	buf := make([]byte, BEAD_BUF_SIZE)
	var totalReadData []byte
	var totalReadSize int

	// Read file
	for {
		readSize, readErr := fp.Read(buf)
		if readSize == 0 {
			break
		}
		if readErr != nil {
			return nil, 0, readErr
		}
		totalReadData = append(totalReadData, buf...)
		totalReadSize += readSize
	}
	// Close file
	closeErr := fp.Close()
	if closeErr != nil {
		return totalReadData, totalReadSize, closeErr
	}

	return totalReadData, totalReadSize, nil
}

func createJoinedFile(partialFiles []string) (string, error) {
	// Create temporary file
	tmpFile, createTmpFileErr := ioutil.TempFile(tmpFileDir, tmpFilePrefix)
	if createTmpFileErr != nil {
		return "", createTmpFileErr
	}

	// Read each partial files and write into tmp file
	for _, partialFilePath := range partialFiles {
		// Read file data
		data, readSize, readFileErr := readFile(partialFilePath)
		if readFileErr != nil {
			return "", readFileErr
		}
		// Truncate data
		data = data[:readSize]

		// Write to tmp file
		if _, writeErr := tmpFile.Write(data); writeErr != nil {
			return "", writeErr
		}
	}
	// Close tmp file
	closeErr := tmpFile.Close()
	if closeErr != nil {
		return "", closeErr
	}

	return tmpFile.Name(), nil
}

func subsets(motherSet []string) (allSubsets [][]string) {
	length := len(motherSet)
	for bit := 0; bit < (1 << uint(length)); bit++ {
		subset := []string{}
		for cur := 0; cur < length; cur++ {
			if bit&(1<<uint(cur)) != 0 {
				subset = append(subset, motherSet[cur])
			}
		}
		allSubsets = append(allSubsets, subset)
	}
	return allSubsets
}

func getCurrentFilePaths(partialFilePaths []string, currentFilePath string) ([]string, error) {
	// Sort partialFilePaths
	sort.Strings(partialFilePaths)
	// Create all subsets of partialFilePaths
	allSubsets := subsets(partialFilePaths)
	// For each subsets
	for _, subset := range allSubsets {
		// Create joined file
		tmpFile, createJoinedFileErr := createJoinedFile(subset)
		if createJoinedFileErr != nil {
			return nil, createJoinedFileErr
		}
		// Read the joined file
		tmpFileData, _, readTmpFileErr := readFile(tmpFile)
		if readTmpFileErr != nil {
			return nil, readTmpFileErr
		}
		// Remove the joined file
		os.Remove(tmpFile)
		// Get md5 digest of the joined file
		tmpFileDigest := md5.Sum(tmpFileData)
		// Read current file
		currentFileData, _, readCurrentFileErr := readFile(currentFilePath)
		if readCurrentFileErr != nil {
			return nil, readCurrentFileErr
		}
		// Get md5 digest of current file
		currentFileDigest := md5.Sum(currentFileData)
		// If current digest and joined file's digest are not equal, check next subset
		if tmpFileDigest != currentFileDigest {
			continue
		}
		return subset, nil
	}
	return nil, nil
}

func main() {

	var args arguments.Args
	args.AddOptions([]option.Option{
		option.Option{
			LongKey:     "file",
			ShortKey:    "f",
			Description: "Specify file you want to create.",
			ValueType:   "string",
			Required:    true,
		},
	})
	if optParseErr := args.Parse(); optParseErr != nil {
		fmt.Println(optParseErr)
		fmt.Println(args)
		return
	}

	clitool.Init()
	defer clitool.Close()

	// Get artifact file path from command line option
	artifactFilePath, getStrErr := args.GetString("file")
	if getStrErr != nil {
		printErrorAndWaitEsc(getStrErr.Error())
		return
	}
	partialFileDir := filepath.Dir(artifactFilePath)
	partialFilePrefix := filepath.Base(artifactFilePath) + "_"

	// Get partial file list (the paths are absolute path)
	partialFilePaths, getPartialFilePathsErr :=
		getPartialFilePaths(partialFileDir, partialFilePrefix)
	// Something error was occurred
	if getPartialFilePathsErr != nil {
		printErrorAndWaitEsc("Failed to get partial file list.")
		return
	}
	// Partial files are not found
	if len(partialFilePaths) <= 0 {
		printErrorAndWaitEsc("Partial files are not found.")
		return
	}

	// Get partial files that is used in current setting
	currentFilePaths, getCurrentFilePathsErr :=
		getCurrentFilePaths(partialFilePaths, artifactFilePath)
	if getCurrentFilePathsErr != nil {
		printErrorAndWaitEsc(getCurrentFilePathsErr.Error())
	}
	if currentFilePaths == nil {
		clitool.Error("Failed to find current setting.")
	}

	// Provide checkbox
	usedPartialFiles := clitool.Checkbox(
		"Please choose all files you want to join with "+artifactFilePath,
		partialFilePaths,
		currentFilePaths,
	)

	// Sort usedPartialFiles
	sort.Strings(usedPartialFiles)

	// Create temporary file
	tmpFilePath, createJoinedFileErr := createJoinedFile(usedPartialFiles)
	if createJoinedFileErr != nil {
		printErrorAndWaitEsc("Failed to create joined config file. " + createJoinedFileErr.Error())
	}

	// Move temporary file to artifact path
	if renameErr := os.Rename(tmpFilePath, artifactFilePath); renameErr != nil {
		os.Remove(tmpFilePath)
		printErrorAndWaitEsc(
			"Failed to rename temporary file to \"" + artifactFilePath + "\"." + renameErr.Error())
		return
	}

	// Change mode of artifact file
	if chmodErr := os.Chmod(artifactFilePath, artifactMode); chmodErr != nil {
		printErrorAndWaitEsc(
			"Failed to change mode \"" + artifactFilePath + "\"." + chmodErr.Error())
		return
	}

	return
}
