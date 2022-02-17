// everything in here copied from ellem.so/lmgo/conf

package main

import (
	"fmt"
	"io/ioutil"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

type ConfigBinding struct {
	Name        string
	VarAddress  interface{}
	Description string
}

var allbinds []ConfigBinding

var validNameRegex = regexp.MustCompile(`^[a-z0-9\-]+$`)

/*
 * Binds VarAddress so that when the config option with Name is loaded in with
 * either LoadConfFile or LoadConfArgs.
 * VarAddress must be an pointer to one of the following datatypes and will be
 * prased in the following ways:
 * - [u]int[8,16,32,64]: parsed in via strconv.Atoi
 * - bool: if "false", "off", "no" then false. if "true", "on", "yes" then true
 * - string: no parsing needed
 * - float(32,64): parsed via ParseFloat
 * - slices: they're comma delimited (for strings requiring a comma, you can wra
 *           quotes around the element like you would with CSV). Each element will be trimmed before parsed to
 *           their respective type.
 *
 * Name must be alpha-numaric all lower case, dashes included.
 * Description is optional but is used to help the user understand what each arg
 *             does.
 * Note: the content to which VarAddress is pointed to will not be modified if
 * no config option is given (via LoadConfArgs and LoadConfFile)
 */
func Bind(varAddress interface{}, name, description string) error {
	b := ConfigBinding{
		name,
		varAddress,
		description,
	}
	if err := b.Validate(); err != nil {
		return lmerrorNew(0x846d,
			"failed to validate",
			err,
			"make sure you're using the proper types/variables",
			name)
	}
	allbinds = append(allbinds, b)
	return nil
}

/*
 * Resets all binds and Adds multiple binds at once. See Bind for details.
 */
func BindAll(binds []ConfigBinding) error {
	allbinds = []ConfigBinding{}
	for _, b := range binds {
		err := Bind(b.VarAddress, b.Name, b.Description)
		if err != nil {
			return err
		}
	}
	return nil
}

/*
 * Loads in a config file and sets all the binded variables to what was found
 * in the config file.
 */
func LoadConfFile(file string) error {
	return LoadConfFileI(file, allbinds)
}

func LoadConfFileI(file string, binding []ConfigBinding) error {
	bytes, err := ioutil.ReadFile(file)
	if err != nil {
		return lmerrorNew(0x8131cacd,
			"failed to open config file",
			err,
			"", "")
	}
	return LoadConfFileContentets(bytes, binding)
}

func LoadConfFileContentets(bytes []byte, binds []ConfigBinding) error {
	contents := string(bytes)
	var ret = make(map[string]int)
	// split it into lines
	//var quoting int = -1 // if -1 that means not quoting. otherwise it will be line number where open quote was
	var variable, value string
	var writeTo *string
	lines := strings.Split(contents, "\n")
	for lineNum := 0; lineNum < len(lines); lineNum++ {
		variable = ""       // everything before the equal sign
		value = ""          // everything after the equal sign
		writeTo = &variable // writeTo will swtich between variable and value

		// walk through each character this given line
		for i := 0; i < len(lines[lineNum]); i++ {
			if lines[lineNum][i] == '=' && writeTo == &variable {
				// found the equal sign, start writing the value
				writeTo = &value
				continue
			}

			// pound sign before the equal sign is a comment, so stop reading
			// the line all together.
			if lines[lineNum][i] == '#' {
				break
			}
			*writeTo = *writeTo + string(lines[lineNum][i])
		}

		// parse all values as case insensitive
		// trim whitespace from either side of the value and the variable.
		variable = strings.ToLower(strings.TrimSpace(variable))
		value = strings.TrimSpace(value)
		// the variable was empty (ie, a comment line), ignore it. otherwise
		// add it to the ret.
		if len(variable) == 0 {
			// do nothing with the variable... it's empty.
			continue
		}

		// make sure no variable is declared twice.
		if alrd, ok := ret[variable]; ok {
			return lmerrorNewf(0x84651d,
				"variable already declared",
				nil,
				"remove duplicate variable declaration",
				"%s declared on lines %d and %d", variable, alrd, lineNum+1)
		}

		// mark this variable as declared
		ret[variable] = lineNum

		// now that we have the variable and it's value in string format.
		// lets go through the binds and asign the variable.
		var j int
		for j = 0; j < len(binds); j++ {
			b := binds[j]
			if b.Name != variable {
				continue
			}

			// found a match between what was bound and what was in conf.
			// do the assign/parsing.
			err := assign(binds[j], value)
			if err != nil {
				return lmerrorNewf(0x87651d,
					"failed to assign value to variable",
					err,
					"make sure you have the right data type/formatting",
					"%s declared on line %d", variable, lineNum+1)
			}
			break
		}
		if j == len(binds) {
			// the variable in the content does not match anything that
			// was binded
			return lmerrorNewf(0x87651d,
				"variable not found",
				nil,
				"remove or comment out the variable",
				"%s declared line %d", variable, lineNum+1)
		}
	}
	return nil
}

/*
 * Parses args as if it was os.Args. Looks at all variables passed in as long
 * operations and assigns them to what was found in bind.
 * Args will be in gnu style. meaning the 'myvar' will be checked
 * against '--myvar'. Additional/extra parameters will be ignored
 * Note: bools must be excplicty set, not treated as 'flags'
 * Note: if you pass in os.Args and your first arg (the executable Name itself)
 *       starts with '--'... expect problems. So dont make executables start
 *       with '--', obviously.
 */
func LoadConfArgs(args []string) error {
	// this function will convert args into a conf file and just reuse
	// LoadConfFile
	var i int
	for i = 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "--") {
			continue
		}
		// it has a -- prefix... see if we can find the bind
		var j int
		var value string
		for j = 0; j < len(allbinds); j++ {
			// handle the '--var=val' notation
			if strings.HasPrefix(a[2:], allbinds[j].Name+"=") {
				parts := strings.SplitN(a[2:], "=", 2)
				value = parts[1]
				break
			}
			if allbinds[j].Name == a[2:] {
				break
			}
		}
		if j == len(allbinds) {
			return lmerrorNew(0x1118,
				"argument not found",
				nil,
				"see help for list of arguments",
				a)
		}

		// okay we found the variable at allbinds[j]
		// first see if the '=' notation didn't already set value
		if value != "" {
			if i+1 == len(args) || strings.HasPrefix(args[i+1], "--") {
				return lmerrorNewf(0x1119,
					"variable was not assigned to a value",
					nil,
					"variables must be given in '--myvar myval' or '--myvar=myval' notation",
					"%s (argument #%d)",
					a, i)
			}
			value = args[i+1]
		}

		err := assign(allbinds[j], value)
		if err != nil {
			return lmerrorNewf(0x87651d,
				"failed to assign value to variable",
				err,
				"make sure you have the right data type/formatting",
				"%s (argument #%d)",
				a, i)
		}
	}
	return nil
}

func GetParameters(args []string) (params []string) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "--") {
			params = append(params, a)
			continue
		}
		parts := strings.SplitN(a, "=", 2)
		if len(parts) == 2 {
			continue
		} else {
			i++
			continue
		}
	}
	return params
}

func (b ConfigBinding) Validate() error {
	// make sure it's not already bound
	for _, c := range allbinds {
		if b.Name == c.Name {
			return lmerrorNewf(0x231c,
				"already bound",
				nil,
				"you've already bound a variable with the same Name",
				"%s", b.Name)
		}
	}

	// make sure the Name is valid
	if !validNameRegex.MatchString(b.Name) {
		return lmerrorNewf(0x231,
			"invalid variable Name",
			nil,
			"variable names must be alphanumaric, all lowercase, dashes included.",
			"%s", b.Name)
	}

	// make sure it's the right type
	varAddress := b.VarAddress
	t := reflect.TypeOf(varAddress)
	if t.Kind() != reflect.Ptr {
		return lmerrorNewf(0x234,
			"not a pointer/slice type",
			nil,
			"you must use only pointer types for configuring",
			"%s is of type %s", b.Name, t.Name())
	}

	// if it is a slice, we need to get the slice type and make sure
	// it's on the list of valid types.
	if t.Elem().Kind() == reflect.Slice {
		varAddress = reflect.New(t.Elem().Elem()).Interface()
	}

	// the whitelist of types.
	switch varAddress.(type) {
	case *int:
	case *int8:
	case *int16:
	case *int32:
	case *int64:
	case *uint:
	case *uint8:
	case *uint16:
	case *uint32:
	case *uint64:
	case *string:
	case *float32:
	case *float64:
	case *bool:
		break
	default:
		return lmerrorNewf(0x237,
			"unsupported type",
			nil,
			"use only ints, strings, floats, and bools",
			"%s is of type %T", b.Name, varAddress)
	}

	// make sure it's not a nil pointer
	isnil := reflect.ValueOf(varAddress).IsNil()
	if isnil {
		return lmerrorNewf(0x2300,
			"nil pointer",
			nil,
			"do not pass in nil pointers into config as config does not manage memory",
			"%s is set to a nil pointer", b.Name)
	}

	return nil
}

func (b ConfigBinding) defaultValue() string {
	varg := reflect.ValueOf(b.VarAddress).Elem().Interface()
	var ret string
	if reflect.ValueOf(b.VarAddress).Elem().Kind() == reflect.Slice {
		vvarg := reflect.ValueOf(b.VarAddress).Elem()
		for i := 0; i < vvarg.Len(); i++ {
			bb := ConfigBinding{
				VarAddress: vvarg.Index(i).Addr().Interface(),
			}
			ret += bb.defaultValue() + ", "
		}
		if vvarg.Len() != 0 {
			// remove the last ", " from the list.
			ret = ret[:len(ret)-2]
		}
		return ret
	} else {
		ret = fmt.Sprintf("%v", varg)
	}

	// make sure replace any '"' that show up as to not invoke quotes.
	ret = strings.ReplaceAll(ret, "\"", "\\\"")

	// if any special charaters are found, or, if the default value will not
	// survive trimming, we need to quote the whole thing
	if strings.IndexAny(ret, "\n#,") != -1 || strings.TrimSpace(ret) != ret {
		return "\"" + ret + "\""
	} else {
		return ret
	}
}

const helpargsIndent = "   "
const helpargsCharlimit = 80

func HelpArgs() string {
	ret := ""
	for i := 0; i < len(allbinds); i++ {
		// Name and default value ie:
		// --myvar=FLOAT
		//   default value is ...
		//   (Description)
		ret += helpargsIndent + "--" + allbinds[i].Name + "=" +
			strings.ToUpper(reflect.TypeOf(allbinds[i].VarAddress).Elem().Name()) + "\n"
		ret += helpargsIndent + "  Default value is \"" + allbinds[i].defaultValue() + "\".\n"

		// Description
		// word-wrap it around the 80 column rule. adding a "  " before
		// each line
		dparts := strings.Split(allbinds[i].Description, " ")
		if len(allbinds[i].Description) == 0 {
			ret += "\n"
			continue
		}
		ret += helpargsIndent + " "
		var linelen int = 1 + len(helpargsIndent) // +1 for " "
		for j := 0; j < len(dparts); j++ {
			ret += " "
			linelen++
			// print the word. if it has a \n in it. reset linelen
			for k := 0; k < len(dparts[j]); k++ {
				if dparts[j][k] == '\n' {
					ret += "\n "
					linelen = 1
				} else {
					ret += string(dparts[j][k])
					linelen++
				}
			}
			if j+1 == len(dparts) || linelen+len(dparts[j+1])+1 > helpargsCharlimit {
				ret += "\n" + helpargsIndent + "  "
				linelen = 1 + len(helpargsIndent)
			}
		}
		ret += "\n"
	}
	return ret
}

func HelpFile() string {
	return HelpFileI(allbinds)
}

func HelpFileI(binds []ConfigBinding) string {
	ret := ""
	for i := 0; i < len(binds); i++ {
		// Description
		// word-wrap it around the 80 column rule. adding a "# " before
		// each line
		dparts := strings.Split(binds[i].Description, " ")
		if len(binds[i].Description) == 0 {
			ret += "\n"
		}
		ret += "#"
		var linelen int = 1 // +1 for "#"
		for j := 0; j < len(dparts); j++ {
			ret += " "
			for k := 0; k < len(dparts[j]); k++ {
				if dparts[j][k] == '\n' {
					ret += "\n#"
					linelen = 1
				} else {
					ret += string(dparts[j][k])
					linelen++
				}
			}
			if j+1 == len(dparts) || linelen+len(dparts[j+1])+1 > helpargsCharlimit {
				ret += "\n#"
				linelen = 1
			}
		}

		// Name and default value ie:
		// #myvar = myval
		ret += binds[i].Name + " = " + binds[i].defaultValue()
		ret += "\n\n"
	}
	return ret
}

// helper function to loadConfcontent
// note: it is assumed that Validate() was called prior to this function.
func assign(binding ConfigBinding, content string) error {
	varAddress := binding.VarAddress

	if reflect.TypeOf(varAddress).Elem().Kind() == reflect.Slice {
		t := reflect.TypeOf(varAddress)
		contents := splitContent(content)
		// reflection magic
		// we make a pointer to a new array and set varAddress to be set to that poiinter.
		varAddressS := reflect.MakeSlice(t.Elem(), len(contents), len(contents))
		for c, cstr := range contents {
			err := assingPtr(varAddressS.Index(c).Addr().Interface(), cstr)
			if err != nil {
				return err
			}
		}
		reflect.ValueOf(varAddress).Elem().Set(varAddressS)
		return nil
	} else {
		return assingPtr(varAddress, content)
	}
}

func splitContent(s string) []string {
	var quoting = false
	strs := strings.Split(s, ",")
	ret := make([]string, 0, len(strs))
	// start reading through the content.
	for c := 0; c < len(strs); c++ {
		// are we quoting?
		if quoting {
			// we are.. so just add the next part raw

			// is the last (trimmed) carachter before
			// the charater a quote?
			ts := strings.TrimRight(strs[c], " ")
			if ts[len(ts)-1] == '"' {
				// if so, we're no longer quoting
				ret[len(ret)-1] += "," + ts[:len(ts)-1]
				quoting = false
				continue
			}

			// raw add
			ret[len(ret)-1] += "," + strs[c]
		} else {
			tleft := strings.TrimLeft(strs[c], " ")
			if len(tleft) > 0 && tleft[0] == '"' {
				// this element starts with a quote.
				// so enter quoting mode.
				ret = append(ret, tleft[1:])
				quoting = true
				continue
			}

			// we are not quoting, nor did we detect a quote.
			// so just add the trimmed element.
			ret = append(ret, strings.TrimSpace(strs[c]))
		}
	}
	return ret
}

func assingPtr(varAddress interface{}, content string) error {
	switch varAddress.(type) {
	case *int:
		v, err := strconv.ParseInt(content, 0, 0)
		if err != nil {
			return lmerrorNew(0x3318,
				"bad int value",
				err,
				"format the int properly",
				content)
		}
		*(varAddress.(*int)) = int(v)
		break
	case *int8:
		v, err := strconv.ParseInt(content, 0, 8)
		if err != nil {
			return lmerrorNew(0x3318,
				"bad int8 value",
				err,
				"format the int properly",
				content)
		}
		*(varAddress.(*int8)) = int8(v)
		break
	case *int16:
		v, err := strconv.ParseInt(content, 0, 16)
		if err != nil {
			return lmerrorNew(0x3318,
				"bad int16 value",
				err,
				"format the int properly",
				content)
		}
		*(varAddress.(*int16)) = int16(v)
		break
	case *int32:
		v, err := strconv.ParseInt(content, 0, 32)
		if err != nil {
			return lmerrorNew(0x3318,
				"bad int32 value",
				err,
				"format the int properly",
				content)
		}
		*(varAddress.(*int32)) = int32(v)
		break
	case *int64:
		v, err := strconv.ParseInt(content, 0, 64)
		if err != nil {
			return lmerrorNew(0x3319,
				"bad int64 value",
				err,
				"format the int properly",
				content)
		}
		*(varAddress.(*int64)) = v
		break
	case *uint:
		v, err := strconv.ParseUint(content, 0, 0)
		if err != nil {
			return lmerrorNew(0x3318,
				"bad uint value",
				err,
				"format the int properly",
				content)
		}
		*(varAddress.(*uint)) = uint(v)
		break
	case *uint8:
		v, err := strconv.ParseUint(content, 0, 8)
		if err != nil {
			return lmerrorNew(0x3318,
				"bad uint8 value",
				err,
				"format the int properly",
				content)
		}
		*(varAddress.(*uint8)) = uint8(v)
		break
	case *uint16:
		v, err := strconv.ParseUint(content, 0, 16)
		if err != nil {
			return lmerrorNew(0x3318,
				"bad uint16 value",
				err,
				"format the int properly",
				content)
		}
		*(varAddress.(*uint16)) = uint16(v)
		break
	case *uint32:
		v, err := strconv.ParseUint(content, 0, 32)
		if err != nil {
			return lmerrorNew(0x3318,
				"bad uint32 value",
				err,
				"format the int properly",
				content)
		}
		*(varAddress.(*uint32)) = uint32(v)
		break
	case *uint64:
		v, err := strconv.ParseUint(content, 0, 64)
		if err != nil {
			return lmerrorNew(0x3319,
				"bad uint64 value",
				err,
				"format the int properly",
				content)
		}
		*(varAddress.(*uint64)) = v
		break
	case *string:
		// easy.
		*(varAddress.(*string)) = content
		break
	case *bool:
		lcontent := strings.ToLower(content)
		switch lcontent {
		case "off":
			fallthrough
		case "false":
			fallthrough
		case "no":
			*(varAddress.(*bool)) = false
			break
		case "on":
			fallthrough
		case "true":
			fallthrough
		case "yes":
			*(varAddress.(*bool)) = true
			break
		default:
			return lmerrorNew(0x331a,
				"bad boolean value",
				nil,
				"boolean values must be set to 'off', 'false', 'no' for false. or 'on', 'true', 'yes' for true",
				content)
		}
		break
	case *float32:
		f, err := strconv.ParseFloat(content, 32)
		if err != nil {
			return lmerrorNew(0x331b,
				"failed to parse float32 value",
				err,
				"make sure float32 value is formatted correctly",
				content)
		}
		*(varAddress.(*float32)) = float32(f)
		break
	case *float64:
		f, err := strconv.ParseFloat(content, 64)
		if err != nil {
			return lmerrorNew(0x331c,
				"failed to parse float64 value",
				err,
				"make sure float64 value is formatted correctly",
				content)
		}
		*(varAddress.(*float64)) = f
		break
	}
	return nil
}
