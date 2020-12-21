package vorlage

import "fmt"

func (p ProcessorInfo) String() string {
	ret := ""
	var args []interface{}
	//Name
	ret += "\t%-28s: %s\n"
	args = append(args, "Name")
	args = append(args, p.Name)

	//Description
	ret += "\t%-28s: %s\n"
	args = append(args, "Description")
	args = append(args, p.Description)

	//inputs
	//if len(p.InputProto) == 0 {
	//	ret += "\tno input needed on request\n"
	//}
	printFormatInputProto(p.InputProto, "\t", "inputs", &ret, &args)
	//if len(p.StreamInputProto) == 0 {
	//	ret += "\tno streams needed on request\n"
	//}
	printFormatInputProto(p.StreamInputProto, "\t", "streams", &ret, &args)

	for _,v := range p.Variables {
		ret += "\t%-28s: %s\n"
		varprefix := fmt.Sprintf("variable[%s]", v.Name)
		args = append(args, varprefix)
		args = append(args, v.Description)
			//inputs
		//if len(p.InputProto) == 0 {
		//	ret += "\tno input needed on request\n"
		//}
		printFormatInputProto(v.InputProto, "\t" + varprefix, "input", &ret, &args)
		//if len(p.StreamInputProto) == 0 {
		//	ret += "\tno streams needed on request\n"
		//}
		printFormatInputProto(v.StreamInputProto, "\t" + varprefix, "stream", &ret, &args)
	}

	str := fmt.Sprintf(ret,args...)
		// remove ending newline
	if str[len(str)-1] == '\n' {
		str = str[0 : len(str)-1]
	}
	return str
}

func printFormatInputProto(p []InputPrototype, prefix string, ty string, ret *string, args *[]interface{}) {
	if len(p) == 0 {
		*ret += prefix + "no " + ty + " requested\n"
		return
	}
	for _,s := range p {
		*ret += "%s%-28s: %s\n"
		*args = append(*args, prefix)
		*args = append(*args, fmt.Sprintf("%s[%s]",ty, s.Name))
		*args = append(*args, s.Description)
	}
}
