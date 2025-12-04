package function

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	krmv1 "github.com/Azure/eno/pkg/krm/functions/api/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var ErrInputNotFound = errors.New("input not found")

type InputReader struct {
	resources *krmv1.ResourceList
}

func NewDefaultInputReader() (*InputReader, error) {
	return NewInputReader(os.Stdin)
}

func NewInputReader(r io.Reader) (*InputReader, error) {
	rl := krmv1.ResourceList{}
	err := json.NewDecoder(r).Decode(&rl)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("decoding stdin as krm resource list: %w", err)
	}
	return &InputReader{
		resources: &rl,
	}, nil
}

// IsOptional returns true if the input with the given key is marked as optional.
// This is determined by checking if any input in the ResourceList has the key and
// the "eno.azure.io/input-optional" annotation set to "true".
func (ir *InputReader) IsOptional(key string) bool {
	for _, item := range ir.resources.Items {
		if getKey(item) == key {
			if anno := item.GetAnnotations(); anno != nil {
				return anno["eno.azure.io/input-optional"] == "true"
			}
			return false
		}
	}
	// Input not found in ResourceList - check if it might be optional by
	// looking at FunctionConfig annotations (for future extension)
	return false
}

func ReadInput[T client.Object](ir *InputReader, key string, out T) error {
	var found bool
	for _, i := range ir.resources.Items {
		i := i
		if getKey(i) == key {
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(i.Object, out)
			if err != nil {
				return fmt.Errorf("converting item to Input: %w", err)
			}
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("input %q: %w", key, ErrInputNotFound)
	}
	return nil
}

func (i *InputReader) All() map[string]*unstructured.Unstructured {
	m := map[string]*unstructured.Unstructured{}
	for _, o := range i.resources.Items {
		m[getKey(o)] = o
	}
	return m
}

func getKey(obj client.Object) string {
	if obj.GetAnnotations() == nil {
		return ""
	}
	return obj.GetAnnotations()["eno.azure.io/input-key"]
}
