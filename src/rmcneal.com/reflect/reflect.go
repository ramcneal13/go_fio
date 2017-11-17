package main

import(
	"fmt"
	"reflect"
)

type myStruct struct {
	number	int
	Str		string
}

func main() {
	fmt.Printf("Starting reflection\n")

	m := myStruct{}
	dumpit(&m)
}

func dumpit(v interface{}) {
	t := reflect.TypeOf(v)
	fmt.Printf("t = %s\n", t.Kind())
	switch t.Kind() {
	case reflect.Ptr:
		//noinspection GoUnusedVariable
		t := reflect.ValueOf(v).Elem()

	case reflect.Struct:
		t := reflect.ValueOf(v).Elem()
		typeOfT := t.Type()
		fmt.Printf("Is a struct\n")
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if f.CanSet() {
				fmt.Printf("%d: %s %s = %v\n", i,
					typeOfT.Field(i).Name, f.Type(), f.Interface())
				dumpit(f.Interface())
			}
		}
	case reflect.Int:
		fmt.Printf("Is Int\n")
	}
}
