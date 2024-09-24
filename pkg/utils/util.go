package utils

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	mrand "math/rand"
	"os"
	"reflect"
	"strings"
	"syscall"
	"text/template"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/rancher/wrangler/v2/pkg/schemas"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	tmpl = `
{{- range $key, $value := .}}
  - {{$key}}: {{$value}}
{{- end}}
`
)

// RandomToken generate random token.
func RandomToken(size int) (string, error) {
	token := make([]byte, size)
	_, err := rand.Read(token)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(token), err
}

// UniqueArray returns unique array.
func UniqueArray(origin []string) (unique []string) {
	unique = make([]string, 0)
	for i := 0; i < len(origin); i++ {
		repeat := false
		for j := i + 1; j < len(origin); j++ {
			if origin[i] == origin[j] {
				repeat = true
				break
			}
		}
		if !repeat {
			unique = append(unique, origin[i])
		}
	}
	return
}

func AskForConfirmationWithError(s string, def bool) (rtn bool, err error) {
	prompt := survey.Confirm{
		Message: s,
		Default: def,
	}
	err = survey.AskOne(&prompt, &rtn)
	return
}

// AskForConfirmation ask for confirmation form os.Stdin.
func AskForConfirmation(s string, def bool) (rtn bool) {
	var err error
	if rtn, err = AskForConfirmationWithError(s, def); err != nil {
		logrus.Warnf("failed to confirm, %v", err)
	}
	return
}

// AskForSelectItem ask for select item from the given map key.
func AskForSelectItem(s string, ss map[string]string) string {
	reader := bufio.NewReader(os.Stdin)
	t := template.New("tmpl")
	t, err := t.Parse(tmpl)
	if err != nil {
		return ""
	}
	buffer := new(bytes.Buffer)
	if err := t.Execute(buffer, ss); err != nil {
		return ""
	}
	fmt.Printf("%s: \n \t%s\n[choose one id]: ", s, buffer.String())
	response, err := reader.ReadString('\n')
	if err != nil {
		logrus.Fatal(err)
	}
	return ss[strings.ToLower(strings.TrimSpace(response))]
}

// WaitFor holds parameters applied to a Backoff function.
func WaitFor(fn func() (bool, error)) error {
	// retry 5 times, total 120 seconds.
	backoff := wait.Backoff{
		Duration: 30 * time.Second,
		Factor:   1,
		Steps:    5,
	}
	return waitForBackoff(fn, backoff)
}

// ConvertToFields convert interface to schemas' field.
func ConvertToFields(obj interface{}) (map[string]schemas.Field, error) {
	t := reflect.TypeOf(obj)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("can't convert non struct type obj %v", obj)
	}
	num := t.NumField()
	fields := make(map[string]schemas.Field, 0)
	for i := 0; i < num; i++ {
		f := t.Field(i)
		if v, ok := f.Tag.Lookup("json"); ok {
			fieldName := strings.Split(v, ",")[0]
			field := schemas.Field{
				Type:    f.Type.String(),
				Default: reflect.ValueOf(obj).Field(i).Interface(),
			}
			fields[fieldName] = field
		}
	}
	return fields, nil
}

// MergeConfig merge config.
func MergeConfig(source, target reflect.Value) {
	if source.Kind() == reflect.Ptr {
		source = source.Elem()
	}
	if target.Kind() == reflect.Ptr {
		target = target.Elem()
	}
	for i := 0; i < source.NumField(); i++ {
		sField := source.Field(i)
		for j := 0; j < target.NumField(); j++ {
			tField := target.Field(j)
			if sField.Type().Kind() == tField.Type().Kind() &&
				source.Type().Field(i).Name == target.Type().Field(j).Name {
				if sField.Type().Kind() == reflect.Struct {
					MergeConfig(sField, tField)
				} else if sField.Type().Kind() == reflect.Bool {
					sField.Set(tField)
				} else {
					// only merge non empty value
					if !tField.IsZero() {
						sField.Set(tField)
					}
				}
				break
			}
		}
	}
}

func waitForBackoff(fn func() (bool, error), backoff wait.Backoff) error {
	return wait.ExponentialBackoff(backoff, func() (bool, error) {
		return fn()
	})
}

func StringSupportBase64(value string) string {
	if value == "" {
		return value
	}
	valueByte, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		logrus.Debugf("failed decode string %s, got error: %v", value, err)
		valueByte = []byte(value)
	}
	return string(valueByte)
}

func GenerateRand() int {
	r := mrand.New(mrand.NewSource(time.Now().UnixNano()))
	return r.Intn(255)
}

func IsTerm() bool {
	return term.IsTerminal(int(syscall.Stdin))
}

func CommandExitWithoutHelpInfo(f func(cmd *cobra.Command, args []string) error) func(cmd *cobra.Command, args []string) {
	return func(cmd *cobra.Command, args []string) {
		if err := f(cmd, args); err != nil {
			cmd.PrintErr(err)
			os.Exit(1)
		}
	}
}
