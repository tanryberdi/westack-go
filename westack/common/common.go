package wst

import (
	"encoding/json"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"io/ioutil"
	"log"
	"regexp"
	"strings"
)

type M map[string]interface{}

func (m M) GetM(key string) M {
	if m == nil {
		return nil
	}
	if v, ok := m[key]; ok {
		if vv, ok := v.(M); ok {
			return vv
		}
	}
	return nil
}

func (m M) GetString(key string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[key]; ok {
		if vv, ok := v.(string); ok {
			return vv
		}
	}
	return ""
}

type A []M

func AFromGenericSlice(in *[]interface{}) *A {

	if in == nil {
		return nil
	}

	out := make(A, len(*in))

	for idx, inDoc := range *in {
		if vv, ok := inDoc.(M); ok {
			out[idx] = vv
		} else {
			log.Println("ERROR: AFromGenericSlice: not an M")
			out[idx] = M{}
		}
	}

	return &out
}

func AFromPrimitiveSlice(in *primitive.A) *A {

	if in == nil {
		return nil
	}

	out := make(A, len(*in))

	for idx, inDoc := range *in {
		if vv, ok := inDoc.(primitive.M); ok {

			out[idx] = M{}
			for k, v := range vv {
				out[idx][k] = v
			}

		} else if vv, ok := inDoc.(M); ok {
			out[idx] = vv
		} else {
			log.Println("ERROR: AFromPrimitiveSlice: not a primitive.M")
			out[idx] = M{}
		}
	}

	return &out
}

type Where M

type IncludeItem struct {
	Relation string  `json:"relation"`
	Scope    *Filter `json:"scope"`
}

type Include []IncludeItem
type Order []string

type Filter struct {
	Where   *Where   `json:"where"`
	Include *Include `json:"include"`
	Order   *Order   `json:"order"`
	Skip    int64    `json:"skip"`
	Limit   int64    `json:"limit"`
}

type IApp struct {
	Debug        bool
	SwaggerPaths func() *map[string]M
	FindModel    func(modelName string) (interface{}, error)
	JwtSecretKey []byte
}

func LoadFile(filePath string, out interface{}) error {
	jsonFile, err := ioutil.ReadFile(filePath)
	if err != nil {
		return err
	}
	err2 := json.Unmarshal(jsonFile, &out)
	if err2 != nil {
		return err2
	}
	return nil
}

func DashedCase(st string) string {
	var res = strings.ToLower(st[:1])
	compile, err := regexp.Compile("([A-Z])")
	if err != nil {
		panic(err)
	}
	res += string(compile.ReplaceAllFunc([]byte(st[1:]), func(bytes []byte) []byte {
		return []byte("-" + strings.ToLower(string(bytes[0])))
	}))
	return res
}

func CopyMap(src M) M {
	targetMap := make(M)
	for key, value := range src {
		targetMap[key] = value
	}
	return targetMap
}

func Transform(in interface{}, out interface{}) error {
	// TODO: move marshal and unmarshal to easyjson
	bytes, err := bson.Marshal(in)
	if err != nil {
		return err
	}
	err2 := bson.Unmarshal(bytes, out)
	if err2 != nil {
		return err2
	}
	return nil
}
