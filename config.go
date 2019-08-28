package oconfig

import (
	"fmt"
	"github.com/qinwei1314ai/xlog"
	"io/ioutil"
	"reflect"
	"strconv"
	"strings"
)

//用户直接传入数据
func UnMarshal(data []byte, result interface{}) (err error) {

	//判断用户传入的是否是指针类型变量
	t := reflect.TypeOf(result)
	v := reflect.ValueOf(result)
	_ = v
	kind := t.Kind()
	if kind != reflect.Ptr {
		panic("please pass a address")
	}

	//定义sectionName，else中处理的都是这个下边的字段
	var sectionName string
	//按行号将data进行分割，逐行进行处理
	lines := strings.Split(string(data), "\n")
	//初始化行号0，每处理一行行号加一
	lineNo := 0
	for _, line := range lines {
		lineNo++
		//去掉特殊字符串：空格  tab  换行符
		line = strings.Trim(line, " \t\r\n")
		if len(line) == 0 {
			continue //空行
		}
		//判断第一个字符如果是#或; 代表注释，跳过
		if line[0] == '#' || line[0] == ';' {
			continue
		}

		//判断第一个字符是‘[’ 表示是一个新section/group
		if line[0] == '[' {
			//判断section是否合法[server]
			if len(line) <= 2 || line[len(line)-1] != ']' {
				tips := fmt.Sprintf("syntax error, invalid section:\"%s\" line:%d", line, lineNo)
				panic(tips)
			}
			//去除section的手尾"[]"字符，获取section名称
			sectionName = strings.TrimSpace(line[1 : len(line)-1])
			//出现[   ] 后，panic
			if len(sectionName) == 0 {
				tips := fmt.Sprintf("syntax error, invalid section:\"%s\" line:%d", line, lineNo)
				panic(tips)
			}

			//xlog.LogDebug("section: %s", sectionName)
		} else {
			//如果没有sectionName，只有key val ，报错panic
			if len(sectionName) == 0 {
				tips := fmt.Sprintf("syntax error, key-value: %s 不属于任何section, lineNo:%d", line, lineNo)
				panic(tips)
			}

			//根据"="分割key-value,
			index := strings.Index(line, "=") //返回第一个"="的索引位置， -1表示未找到
			if index == -1 {
				tips := fmt.Sprintf("syntax error, not found =, line: %s, lineNo:%d", line, lineNo)
				panic(tips)
			}

			key := strings.TrimSpace(line[0:index])
			value := strings.TrimSpace(line[index+1:])

			if len(key) == 0 {
				tips := fmt.Sprintf("syntax error, key is null, line:%s, lineNo:%d", line, lineNo)
				panic(tips)
			}

			//1.通过sectionName找到在result中对应的结构体s1
			//查找result中的结构体字段，根据tag找到对应字段并把section赋值给对应字段;用户传过来的是指针类型变量，通过Elem()寻址
			for i := 0; i < t.Elem().NumField(); i++ {
				tfield := t.Elem().Field(i)
				vField := v.Elem().Field(i)
				if tfield.Tag.Get("ini") != sectionName {
					continue
				}

				//2.通过key找到对应结构体s1中的字段
				//判断field类型是否为结构体
				fieldType := tfield.Type
				if fieldType.Kind() != reflect.Struct {
					tips := fmt.Sprintf("field %s is not struct", tfield.Name)
					panic(tips)
				}

				//field是结构体，继续查找对应字段并赋值
				for j := 0; j < fieldType.NumField(); j++ {
					tKeyField := fieldType.Field(j)
					vKeyField := vField.Field(j)
					if tKeyField.Tag.Get("ini") != key {
						continue
					}

					//找到了子结构体中的字段,判断字段类型并赋值
					switch tKeyField.Type.Kind() {
					//字符串
					case reflect.String:
						vKeyField.SetString(value)
					//整数
					case reflect.Int, reflect.Uint, reflect.Int16, reflect.Uint16:
						fallthrough //直接到下一个case语句
					case reflect.Int32, reflect.Uint32, reflect.Int64, reflect.Uint64:
						//将字符串转换成数字并赋值
						valueInt, err := strconv.ParseInt(value, 10, 64)
						if err != nil {
							tips := fmt.Sprintf("value:%s can not convert to int64, lineNo:%d", value, lineNo)
							panic(tips)
						}
						vKeyField.SetInt(valueInt)
					//浮点数
					case reflect.Float32, reflect.Float64:
						valueFloat, err := strconv.ParseFloat(value, 64)
						if err != nil {
							tips := fmt.Sprintf("value:%s can not convert to floa64, lineNo:%d", value, lineNo)
							panic(tips)
						}
						vKeyField.SetFloat(valueFloat)
					//其他类型直接panic
					default:
						tips := fmt.Sprintf("key:\"%s\" can not convert to %v, lineNo：%d",
							key, tKeyField.Type.Kind(), lineNo)
						panic(tips)
					}
				}
				//找到key并赋值后跳出循环
				break
			}

		}

		//xlog.LogDebug("line: %v", line)

	}

	return
}

//用户传入一个文件
func UnMarshalFile(filename string, result interface{}) (err error) {

	//读取配置文件内容，调用UnMarshal函数
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		xlog.LogError("Read config file err: %v", err)
		return
	}
	return UnMarshal(data, result)
}

func Marshal(result interface{}) (data []byte, err error) {

	t := reflect.TypeOf(result)
	v := reflect.ValueOf(result)
	if t.Kind() != reflect.Struct {
		tips := "please input struct type"
		panic(tips)
	}

	var strSlice []string
	for i := 0; i < t.NumField(); i++ {
		tField := t.Field(i)
		vField := v.Field(i)
		if tField.Type.Kind() != reflect.Struct {
			continue
		}

		sectionName := tField.Name
		if len(tField.Tag.Get("ini")) > 0 {
			sectionName = tField.Tag.Get("ini")
		}
		sectionName = fmt.Sprintf("[%s]\n", sectionName)
		strSlice = append(strSlice, sectionName)

		for j := 0; j < tField.Type.NumField(); j++ {

			subTField := tField.Type.Field(j)
			if subTField.Type.Kind() == reflect.Struct || subTField.Type.Kind() == reflect.Ptr {
				//跳过结构体字段
				continue
			}
			subTFieldName := subTField.Name
			if len(subTField.Tag.Get("ini")) > 0 {
				subTFieldName = subTField.Tag.Get("ini")
			}
			subVField := vField.Field(j)
			fieldStr := fmt.Sprintf("%s=%v\n", subTFieldName, subVField.Interface())
			xlog.LogDebug("conf:%s", fieldStr)

			strSlice = append(strSlice, fieldStr)
		}
	}

	for _, v := range strSlice {
		data = append(data, []byte(v)...)
	}
	return
}

func MarshalFile(filename string, result interface{}) (err error) {

	data, err := Marshal(result)
	if err != nil {
		return
	}

	err = ioutil.WriteFile(filename, data, 0644)
	return
}
