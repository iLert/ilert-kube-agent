package commander

import "regexp"

var ErrorMatchers errorMatchers

const (
	ErrorPodNotFound = "Pod not found"
)

type errorMatchers struct {
	PodNotFound *regexp.Regexp
}

func (matchers *errorMatchers) Init() error {
	var err error
	matchers.PodNotFound, err = regexp.Compile(`^pods "[^"]*" not found$`) // pods "web-s" not found
	if err != nil {
		return err
	}
	return nil
}
