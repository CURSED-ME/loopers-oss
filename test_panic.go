package main

func main() {
	var x interface{} = nil
	s, ok := x.(string)
	println(s, ok)
	s2, _ := x.(string)
	println(s2)
}
