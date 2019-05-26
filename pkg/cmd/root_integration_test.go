package cmd_test

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/stefanoj3/dirstalk/pkg/cmd"
	"github.com/stefanoj3/dirstalk/pkg/common/test"
	"github.com/stretchr/testify/assert"
)

func TestRootCommand(t *testing.T) {
	logger, _ := test.NewLogger()

	c, err := createCommand(logger)
	assert.NoError(t, err)
	assert.NotNil(t, c)

	_, out, err := executeCommand(c)
	assert.NoError(t, err)

	// ensure the summary is printed
	assert.Contains(t, out, "dirstalk is a tool that attempts")
	assert.Contains(t, out, "Usage")
	assert.Contains(t, out, "dictionary.generate")
	assert.Contains(t, out, "scan")
}

func TestScanCommand(t *testing.T) {
	logger, _ := test.NewLogger()

	c, err := createCommand(logger)
	assert.NoError(t, err)
	assert.NotNil(t, c)

	var calls int32
	srv := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&calls, 1)
			w.WriteHeader(http.StatusNotFound)
		}),
	)
	defer srv.Close()

	_, _, err = executeCommand(c, "scan", srv.URL, "--dictionary", "testdata/dict.txt")
	assert.NoError(t, err)

	assert.Equal(t, int32(3), calls)
}

func TestScanWithRemoteDictionary(t *testing.T) {
	logger, _ := test.NewLogger()

	c, err := createCommand(logger)
	assert.NoError(t, err)
	assert.NotNil(t, c)

	dictionaryServer := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			dict := `home
home/index.php
blabla
`
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(dict))
		}),
	)
	defer dictionaryServer.Close()

	var calls int32
	srv := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&calls, 1)
			w.WriteHeader(http.StatusNotFound)
		}),
	)
	defer srv.Close()

	_, _, err = executeCommand(c, "scan", srv.URL, "--dictionary", dictionaryServer.URL)
	assert.NoError(t, err)

	assert.Equal(t, int32(3), calls)
}

func TestScanWithUserAgentFlag(t *testing.T) {
	const testUserAgent = "my_test_user_agent"

	logger, _ := test.NewLogger()

	c, err := createCommand(logger)
	assert.NoError(t, err)
	assert.NotNil(t, c)

	var callsWithMatchingUserAgent int32
	var callsWithNonMatchingUserAgent int32

	srv := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("User-Agent") == testUserAgent {
				atomic.AddInt32(&callsWithMatchingUserAgent, 1)
			} else {
				atomic.AddInt32(&callsWithNonMatchingUserAgent, 1)
			}

			w.WriteHeader(http.StatusNotFound)
		}),
	)
	defer srv.Close()

	_, _, err = executeCommand(
		c,
		"scan",
		srv.URL,
		"--user-agent",
		testUserAgent,
		"--dictionary",
		"testdata/dict.txt",
	)
	assert.NoError(t, err)

	assert.Equal(t, int32(3), callsWithMatchingUserAgent)
	assert.Equal(t, int32(0), callsWithNonMatchingUserAgent)
}

func TestDictionaryGenerateCommand(t *testing.T) {
	logger, _ := test.NewLogger()

	c, err := createCommand(logger)
	assert.NoError(t, err)
	assert.NotNil(t, c)

	testFilePath := "testdata/" + test.RandStringRunes(10)
	defer removeTestFile(testFilePath)
	_, _, err = executeCommand(c, "dictionary.generate", ".", "-o", testFilePath)
	assert.NoError(t, err)

	content, err := ioutil.ReadFile(testFilePath)
	assert.NoError(t, err)

	// Ensure the command ran and produced some of the expected output
	// it is not in the scope of this test to ensure the correct output
	assert.Contains(t, string(content), "root_integration_test.go")
}

func TestGenerateDictionaryWithoutOutputPath(t *testing.T) {
	logger, _ := test.NewLogger()

	c, err := createCommand(logger)
	assert.NoError(t, err)
	assert.NotNil(t, c)

	_, _, err = executeCommand(c, "dictionary.generate", ".")
	assert.NoError(t, err)
}

func TestGenerateDictionaryWithInvalidDirectory(t *testing.T) {
	logger, _ := test.NewLogger()

	fakePath := "./" + test.RandStringRunes(10)
	c, err := createCommand(logger)
	assert.NoError(t, err)
	assert.NotNil(t, c)

	_, _, err = executeCommand(c, "dictionary.generate", fakePath)
	assert.Error(t, err)

	assert.Contains(t, err.Error(), "unable to use the provided path")
	assert.Contains(t, err.Error(), fakePath)
}

func TestVersionCommand(t *testing.T) {
	logger, buf := test.NewLogger()

	c, err := createCommand(logger)
	assert.NoError(t, err)
	assert.NotNil(t, c)

	_, _, err = executeCommand(c, "version")
	assert.NoError(t, err)

	// Ensure the command ran and produced some of the expected output
	// it is not in the scope of this test to ensure the correct output
	assert.Contains(t, buf.String(), "Version: ")
}

func executeCommand(root *cobra.Command, args ...string) (c *cobra.Command, output string, err error) {
	buf := new(bytes.Buffer)
	root.SetOutput(buf)

	a := []string{""}
	os.Args = append(a, args...)

	c, err = root.ExecuteC()

	return c, buf.String(), err
}

func removeTestFile(path string) {
	if !strings.Contains(path, "testdata") {
		return
	}

	_ = os.Remove(path)
}

func createCommand(logger *logrus.Logger) (*cobra.Command, error) {
	dirStalkCmd, err := cmd.NewRootCommand(logger)
	if err != nil {
		return nil, err
	}

	scanCmd, err := cmd.NewScanCommand(logger)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create scan command")
	}

	dirStalkCmd.AddCommand(scanCmd)
	dirStalkCmd.AddCommand(cmd.NewGenerateDictionaryCommand())
	dirStalkCmd.AddCommand(cmd.NewVersionCommand(logger.Out))

	return dirStalkCmd, nil
}
