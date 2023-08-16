package common

import (
	"bytes"
	"fmt"
	"os"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/cnrancher/autok3s/pkg/utils"
	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/strvals"

	"k8s.io/apimachinery/pkg/util/validation"
)

func GetManifest(manifestFile, manifestContent string) (string, error) {
	if manifestFile != "" {
		fileByte, err := os.ReadFile(manifestFile)
		if err != nil {
			return "", err
		}
		return string(fileByte), nil
	} else if manifestContent != "" {
		return utils.StringSupportBase64(manifestContent), nil
	}
	return "", errors.New("can't get manifest with empty manifest file or content")
}

func GenerateValues(setValues map[string]string, defaultValues map[string]string) (map[string]interface{}, error) {
	values := []string{}
	for key, value := range defaultValues {
		if _, ok := setValues[key]; !ok {
			values = append(values, fmt.Sprintf("%s=%s", key, value))
		}
	}
	for key, value := range setValues {
		values = append(values, fmt.Sprintf("%s=%s", key, value))
	}
	return mergeValues(values)
}

func AssembleManifest(values map[string]interface{}, manifest string, templateFunc template.FuncMap) ([]byte, error) {
	t := template.New("manifest").Funcs(sprig.TxtFuncMap())
	if templateFunc != nil {
		t = t.Funcs(templateFunc)
	}
	t, err := t.Parse(manifest)
	if err != nil {
		return nil, err
	}
	var resultContent bytes.Buffer
	if err := t.Execute(&resultContent, values); err != nil {
		return nil, err
	}
	return resultContent.Bytes(), nil
}

func mergeValues(values []string) (map[string]interface{}, error) {
	base := map[string]interface{}{}

	for _, value := range values {
		if err := strvals.ParseInto(value, base); err != nil {
			return nil, errors.Wrap(err, "failed parsing --set data")
		}
	}
	return base, nil
}

func ValidateName(name string) error {
	if name == "" {
		return errors.New("name is required for addon creation")
	}
	if _, err := DefaultDB.GetAddon(name); err == nil {
		return fmt.Errorf("addon %s is already exist", name)
	}
	if errs := validation.IsDNS1123Subdomain(name); len(errs) > 0 {
		return fmt.Errorf("name is not validated %s, %v", name, errs)
	}

	return nil
}
