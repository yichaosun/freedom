package freedom

import (
	"fmt"

	"github.com/BurntSushi/toml"
)

var (
	// profileFallbackSearchDirs is a series of directory that is used to search
	// profile file if a profile file has not been found in other directory.
	profileFallbackSearchDirs = []string{"./conf", "./server/conf"}
)

var _ Configurator = (*fallbackConfigurator)(nil)

// detectProfileInFallbackSearchDirs accepts a string with the name of a
// profile file, and search the file in profileFallbackSearchDirs. It returns
// (a string with the path of the profile file, true) if the profile file has
// been found, and ("", false) otherwise.
func detectProfileInFallbackSearchDirs(file string) (string, bool) {
	for _, dir := range profileFallbackSearchDirs {
		filePath := JoinPath(dir, file)
		if IsDir(dir) && IsFile(filePath) {
			return filePath, true
		}
	}

	return "", false
}

// detectProfilePath accepts a string with the name of a profile file, and
// search the file in the directory which specified in environment variable.
// If the file has not been found, continue search the file by
// detectProfileInFallbackSearchDirs. It returns (a string with the path of
// the profile file, true) if the profile file has found, and ("", false)
// otherwise.
func detectProfilePath(file string) (string, bool) {
	dir := ProfileDirFromEnv()

	filePath := JoinPath(dir, file)
	if IsFile(filePath) {
		return filePath, true
	}

	return detectProfileInFallbackSearchDirs(file)
}

// ReadProfile accepts a string with the name of a profile file, and search
// the file by detectProfilePath. It will fill v with the configuration by
// parsing the profile into toml format, and returns nil if the file has
// found. It returns error, if the file has not been found or any error
// encountered.
func ReadProfile(file string, v interface{}) error {
	filePath, isFilePathExist := detectProfilePath(file)

	if !isFilePathExist {
		return fmt.Errorf("file does not exist: %s", filePath)
	}

	_, err := toml.DecodeFile(filePath, v)
	if err != nil {
		Logger().Errorf("[Freedom] configuration decode error: %s", err.Error())
		return err
	}

	Logger().Infof("[Freedom] configuration was found: %s", filePath)
	return nil
}

// fallbackConfigurator is used to act as a fallback if no any configurator
// are applied. It implements Configurator.
type fallbackConfigurator struct{}

// newFallbackConfigurator creates a fallbackConfigurator
func newFallbackConfigurator() *fallbackConfigurator {
	return &fallbackConfigurator{}
}

// Configured proxy a call to ReadProfile
func (*fallbackConfigurator) Configure(obj interface{}, file string, metaData ...interface{}) error {
	return ReadProfile(file, obj)
}
