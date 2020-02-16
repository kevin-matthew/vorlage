package daemon

import (
	"io/ioutil"
	"strings"
)


func ParseConfFile(file string) (map[string]string, error) {
	bytes,err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	config := ParseConf(string(bytes))
	return config,nil
}

func ParseConf(contents string) (map[string]string) {
	ret := make(map[string]string);
	lines := strings.Split(contents,"\n")
	for _,l := range lines {
		// everything before the colon
		variable := ""
		value    := ""
		var writeTo *string = &variable
		for i := 0; i < len(l); i++ {
			if l[i] == '=' {
				writeTo = &value
				continue;
			}
			if l[i] == '#' {
				break;
			}
			*writeTo = *writeTo + string(l[i]);
		}
		variable = strings.ToLower(strings.TrimSpace(variable))
		value    = strings.TrimSpace(value)
		if len(variable) != 0 {
			ret[variable] = value
		}
	}
	return ret
}