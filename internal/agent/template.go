package agent

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

func renderTemplate(tmplStr string, data any) (string, error) {
	v := reflect.ValueOf(data)
	if v.Kind() != reflect.Struct {
		return "", fmt.Errorf("template data must be a struct")
	}
	t := v.Type()

	pairs := make([]string, 0, t.NumField()*2)
	for i := range t.NumField() {
		key := "{{." + t.Field(i).Name + "}}"
		field := v.Field(i)
		var val string
		switch field.Kind() {
		case reflect.String:
			val = field.String()
		case reflect.Int, reflect.Int64:
			val = strconv.FormatInt(field.Int(), 10)
		default:
			val = fmt.Sprint(field.Interface())
		}
		pairs = append(pairs, key, val)
	}
	return strings.NewReplacer(pairs...).Replace(tmplStr), nil
}
