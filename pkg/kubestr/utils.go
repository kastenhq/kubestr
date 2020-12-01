package kubestr

import (
	"fmt"
	"os"
)

const (
	// ErrorColor formatted color red
	ErrorColor = "\033[1;31m%s\033[0m"
	// SuccessColor formatted color green
	SuccessColor = "\033[1;32m%s\033[0m"
	// YellowColor formatted color yellow
	YellowColor = "\033[1;33m%s\033[0m"
)

// Status is a generic structure to return a status
type Status struct {
	StatusCode    StatusCode
	StatusMessage string
	Raw           interface{} `json:",omitempty"`
}

// StatusCode type definition
type StatusCode string

const (
	// StatusOK is the success status code
	StatusOK = StatusCode("OK")
	// StatusWarning is the informational status code
	StatusWarning = StatusCode("Warning")
	// StatusError is the failure status code
	StatusError = StatusCode("Error")
	// StatusInfo is the Info status code
	StatusInfo = StatusCode("Info")
)

// Print prints a status message with a given prefix
func (s *Status) Print(prefix string) {
	switch s.StatusCode {
	case StatusOK:
		printSuccessMessage(prefix + s.StatusMessage)
	case StatusError:
		printErrorMessage(prefix + s.StatusMessage)
	case StatusWarning:
		printWarningMessage(prefix + s.StatusMessage)
	default:
		printInfoMessage(prefix + s.StatusMessage)
	}
}

// printErrorMessage prints the error message
func printErrorMessage(errorMesg string) {
	fmt.Printf("%s  -  ", errorMesg)
	fmt.Printf(ErrorColor, "Error")
	fmt.Println()
}

// printSuccessMessage prints the success message
func printSuccessMessage(message string) {
	fmt.Printf("%s  -  ", message)
	fmt.Printf(SuccessColor, "OK")
	fmt.Println()
}

func printSuccessColor(message string) {
	fmt.Printf(SuccessColor, message)
	fmt.Println()
}

// printInfoMessage prints a warning
func printInfoMessage(message string) {
	fmt.Println(message)
}

// printWarningMessage prints a warning
func printWarningMessage(message string) {
	fmt.Printf(YellowColor+"\n", message)
}

// TestOutput is the generic return value for tests
type TestOutput struct {
	TestName string
	Status   []Status
	Raw      interface{} `json:",omitempty"`
}

// Print prints a TestRetVal as a string output
func (t *TestOutput) Print() {
	fmt.Println(t.TestName + ":")
	for _, status := range t.Status {
		status.Print("  ")
	}
}

func MakeTestOutput(testname string, code StatusCode, mesg string, raw interface{}) *TestOutput {
	return &TestOutput{
		TestName: testname,
		Status:   []Status{makeStatus(code, mesg, nil)},
		Raw:      raw,
	}
}

func makeStatus(code StatusCode, mesg string, raw interface{}) Status {
	return Status{
		StatusCode:    code,
		StatusMessage: mesg,
		Raw:           raw,
	}
}

func convertSetToSlice(set map[string]struct{}) []string {
	var slice []string
	for i := range set {
		slice = append(slice, i)
	}
	return slice
}

// getPodNamespace gets the pods namespace or returns default
func getPodNamespace() string {
	if val, ok := os.LookupEnv(PodNamespaceEnvKey); ok {
		return val
	}
	return DefaultNS
}
