package main

var l = 0
func lol() (int,error) {
	l++
	return l, nil
}

func main() {
	for ll,err := lol(); ll < 4; {
		if err != nil {
			println("asdf")
		}
	}
}