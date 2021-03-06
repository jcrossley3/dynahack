package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	context = ""
)

var (
	dynamicClient dynamic.Interface
)

func buffer(file *os.File) []byte {
	var size int64 = bytes.MinRead
	if fi, err := file.Stat(); err == nil {
		size = fi.Size()
	}
	return make([]byte, size)
}

func decode(in chan []byte, out chan unstructured.Unstructured) {
	for buf := range in {
		spec := unstructured.Unstructured{}
		err := yaml.NewYAMLToJSONDecoder(bytes.NewReader(buf)).Decode(&spec)
		if err != nil {
			if err != io.EOF {
				fmt.Println("ERROR", spec.GetName(), err)
			}
			continue
		}
		out <- spec
	}
	close(out)
}

func read(file *os.File, sink chan []byte) {
	buf := buffer(file)
	manifests := yaml.NewDocumentDecoder(file)
	defer manifests.Close()
	for {
		size, err := manifests.Read(buf)
		if err == io.EOF {
			break
		}
		b := make([]byte, size)
		copy(b, buf)
		sink <- b
	}
	close(sink)
}

func parse(file *os.File) []unstructured.Unstructured {
	in, out := make(chan []byte, 10), make(chan unstructured.Unstructured, 10)
	go read(file, in)
	go decode(in, out)
	result := []unstructured.Unstructured{}
	for spec := range out {
		result = append(result, spec)
	}
	return result
}

func pluralize(kind string) string {
	ret := strings.ToLower(kind)
	switch {
	case strings.HasSuffix(ret, "s"):
		return fmt.Sprintf("%ses", ret)
	case strings.HasSuffix(ret, "policy"):
		return fmt.Sprintf("%sies", ret[:len(ret)-1])
	default:
		return fmt.Sprintf("%ss", ret)
	}
}

func client(spec unstructured.Unstructured) (dynamic.ResourceInterface, error) {
	groupVersion, err := schema.ParseGroupVersion(spec.GetAPIVersion())
	if err != nil {
		return nil, err
	}
	groupVersionResource := groupVersion.WithResource(pluralize(spec.GetKind()))
	fmt.Println(groupVersionResource)
	if ns := spec.GetNamespace(); ns == "" {
		return dynamicClient.Resource(groupVersionResource), nil
	} else {
		return dynamicClient.Resource(groupVersionResource).Namespace(ns), nil
	}
}

func createResources(resources []unstructured.Unstructured) error {
	for _, spec := range resources {
		c, err := client(spec)
		if err != nil {
			return err
		}
		_, err = c.Create(&spec, v1.CreateOptions{})
		if err != nil {
			fmt.Println("ERROR", spec.GetName(), err)
		}
	}
	return nil
}

func deleteResources(resources []unstructured.Unstructured) error {
	a := make([]unstructured.Unstructured, len(resources))
	copy(a, resources)
	for left, right := 0, len(a)-1; left < right; left, right = left+1, right-1 {
		a[left], a[right] = a[right], a[left]
	}
	for _, spec := range a {
		c, err := client(spec)
		if err != nil {
			return err
		}
		err = c.Delete(spec.GetName(), &v1.DeleteOptions{})
		if err != nil {
			fmt.Println("ERROR", spec.GetName(), err)
		}
	}
	return nil
}

func getResources(resources []unstructured.Unstructured) error {
	for _, spec := range resources {
		c, err := client(spec)
		if err != nil {
			return err
		}
		res, err := c.Get(spec.GetName(), v1.GetOptions{})
		if err != nil {
			fmt.Println("ERROR", spec.GetName(), err)
		} else {
			fmt.Println(res)
		}
	}
	return nil
}

func init() {
	fmt.Printf("Connecting to Kubernetes Context %v\n", context)
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{CurrentContext: context}).ClientConfig()
	if err != nil {
		panic(err.Error())
	}
	dynamicClient, err = dynamic.NewForConfig(config)
}

func usage() {
	fmt.Printf("Usage: %s filename [get|create|delete]\n", os.Args[0])
}

func main() {
	if len(os.Args) == 1 {
		usage()
		return
	}
	file, err := os.Open(os.Args[1])
	if err != nil {
		panic(err.Error())
	}
	cmd := ""
	if len(os.Args) > 2 {
		cmd = os.Args[2]
	}
	switch cmd {
	case "get":
		err = getResources(parse(file))
	case "create":
		err = createResources(parse(file))
	case "delete":
		err = deleteResources(parse(file))
	case "":
		for _, spec := range parse(file) {
			fmt.Println(spec)
		}
	default:
		usage()
	}
	if err != nil {
		panic(err.Error())
	}
}
