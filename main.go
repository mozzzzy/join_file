package main

/*
 * Module Dependencies
 */

import (
	"crypto/md5"
	"io/ioutil"
	"os"
	"sort"
	"strings"

	"github.com/mozzzzy/clitool"
	"github.com/mozzzzy/clitool/checkbox"
	"github.com/mozzzzy/clitool/errorMessage"
	"github.com/mozzzzy/clitool/message"
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
	artifactFilePath  string      = "/etc/hosts"
	artifactMode      os.FileMode = 0644
	partialFileDir    string      = "/etc/"
	partialFilePrefix string      = "hosts_"
	tmpFileDir        string      = "/tmp"
	tmpFilePrefix     string      = "join_file."
)

/*
 * Functions
 */

func getPartialFilePaths(partialFileDir, partialFilePrefix string) ([]string, error) {
	var partialFiles []string
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
		partialFiles = append(partialFiles, file.Name())
	}
	return partialFiles, nil
}

func printError(msg string) {
	err := errorMessage.New(msg)
	clitool.Print(err)
}

func printErrorAndWaitEsc(msg string) {
	printError(msg)
	message := message.New("Please esc key to exit.")
	clitool.Print(message)
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
		data, readSize, readFileErr := readFile(partialFileDir + partialFilePath)
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
	clitool.Init()
	defer clitool.Close()

	// Get partial file list
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
		printError("Failed to find current setting.")
	}

	// Provide checkbox
	chkbox := checkbox.New(
		"Please choose all files you want to join with "+artifactFilePath, partialFilePaths)
	chkbox.Check(currentFilePaths)
	usedPartialFiles := clitool.Inquire(chkbox).([]string)

	// Sort usedPartialFiles
	sort.Strings(usedPartialFiles)

	// Create temporary file
	tmpFilePath, createJoinedFileErr := createJoinedFile(usedPartialFiles)
	if createJoinedFileErr != nil {
		printErrorAndWaitEsc("Failed to create joined config file. " + createJoinedFileErr.Error())
	}

	// Move temporary file to artifact path
	if renameErr := os.Rename(tmpFilePath, artifactFilePath); renameErr != nil {
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
