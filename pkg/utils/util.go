package utils

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"reflect"
	"strings"
	"text/template"
	"time"

	"github.com/rancher/wrangler/pkg/schemas"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	tmpl = `
{{- range $key, $value := .}}
  - {{$key}}: {{$value}}
{{- end}}
`
)

func RandomToken(size int) (string, error) {
	token := make([]byte, size)
	_, err := rand.Read(token)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(token), err
}

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

func AskForConfirmation(s string) bool {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("%s [y/n]: ", s)
		response, err := reader.ReadString('\n')
		if err != nil {
			logrus.Fatal(err)
		}
		response = strings.ToLower(strings.TrimSpace(response))
		if response == "y" || response == "yes" {
			return true
		} else if response == "n" || response == "no" {
			return false
		}
	}
}

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

func WaitFor(fn func() (bool, error)) error {
	// retry 5 times, total 120 seconds.
	backoff := wait.Backoff{
		Duration: 30 * time.Second,
		Factor:   1,
		Steps:    5,
	}
	return WaitForBackoff(fn, backoff)
}

func WaitForBackoff(fn func() (bool, error), backoff wait.Backoff) error {
	if err := wait.ExponentialBackoff(backoff, func() (bool, error) {
		return fn()
	}); err != nil {
		return err
	}
	return nil
}

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
