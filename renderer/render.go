package renderer

import (
	"bytes"
	"reflect"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig"
	"github.com/sirupsen/logrus"
	"github.com/reefbarman/render/files"
	"github.com/reefbarman/render/renderer/configuration"
	"github.com/pkg/errors"
)

const (
	// MissingKeyInvalidOption is the renderer option to continue execution on missing key and print "<no value>"
	MissingKeyInvalidOption = "missingkey=invalid"
	// MissingKeyErrorOption is the renderer option to stops execution immediately with an error on missing key
	MissingKeyErrorOption = "missingkey=error"
	// LeftDelim is the default left template delimiter
	LeftDelim = "{{"
	// RightDelim is the default right template delimiter
	RightDelim = "}}"
)

// Renderer structure holds configuration and options
type Renderer struct {
	configuration configuration.Configuration
	options       []string
	leftDelim     string
	rightDelim    string
}

// New creates a new renderer with the specified configuration and zero or more options
func New(configuration configuration.Configuration, opts ...string) *Renderer {
	return &Renderer{
		configuration: configuration,
		options:       opts,
		leftDelim:     LeftDelim,
		rightDelim:    RightDelim,
	}
}

// Delim mutates Renderer with new left and right delimiters
func (r *Renderer) Delim(left, right string) *Renderer {
	r.leftDelim = left
	r.rightDelim = right
	return r
}

// SimpleRender is a simple rendering function, also used as a custom template function
// to allow in-template recursive rendering, see also Render, RenderWith
func (r *Renderer) SimpleRender(rawTemplate string) (string, error) {
	return r.Render("nameless", rawTemplate)
}

// TODO DirRender

// FileRender is used to render files by path, see also Render
func (r *Renderer) FileRender(inputPath, outputPath string) error {
	input, err := files.ReadInput(inputPath)
	if err != nil {
		logrus.Debugf("Can't open the template: %v", err)
		return err
	}

	var templateName string
	if inputPath == "" {
		templateName = "stdin"
	} else {
		templateName = inputPath
	}

	result, err := r.Render(templateName, string(input))
	if err != nil {
		return err
	}

	err = files.WriteOutput(outputPath, []byte(result), 0644)
	if err != nil {
		logrus.Debugf("Can't save the rendered: %v", err)
		return err
	}

	return nil
}

// Render is the main rendering function, see also SimpleRender, Configuration and ExtraFunctions
func (r *Renderer) Render(templateName, rawTemplate string) (string, error) {
	err := r.Validate()
	if err != nil {
		logrus.Errorf("Invalid state; %v", err)
		return "", err
	}
	t, err := r.Parse(templateName, rawTemplate, r.ExtraFunctions())
	if err != nil {
		logrus.Errorf("Can't parse the template; %v", err)
		return "", err
	}
	out, err := r.Execute(t)
	if err != nil {
		logrus.Errorf("Can't execute the template; %v", err)
		return "", err
	}
	return out, nil
}

// Validate checks the internal state and returns error if necessary
func (r *Renderer) Validate() error {
	if r.configuration != nil {
		err := r.configuration.Validate()
		if err != nil {
			return err
		}
	} else {
		return errors.New("unexpected 'nil' configuration")
	}

	if len(r.leftDelim) == 0 {
		return errors.New("unexpected empty leftDelim")
	}
	if len(r.rightDelim) == 0 {
		return errors.New("unexpected empty rightDelim")
	}

	for _, o := range r.options {
		switch o {
		case MissingKeyErrorOption:
		case MissingKeyInvalidOption:
		default:
			return errors.Errorf("unexpected option: '%s', option must be in: '%s'",
				o, strings.Join([]string{MissingKeyInvalidOption, MissingKeyErrorOption}, ", "))
		}
	}
	return nil
}

// Parse is a basic template parsing function
func (r *Renderer) Parse(templateName, rawTemplate string, extraFunctions template.FuncMap) (*template.Template, error) {
	return template.New(templateName).
		Delims(r.leftDelim, r.rightDelim).
		Funcs(extraFunctions).
		Option(r.options...).
		Parse(rawTemplate)
}

// Execute is a basic template execution function
func (r *Renderer) Execute(t *template.Template) (string, error) {
	var buffer bytes.Buffer
	err := t.Execute(&buffer, r.configuration)
	if err != nil {
		retErr := err
		logrus.Debugf("(%v): %v", reflect.TypeOf(err), err)
		if e, ok := err.(template.ExecError); ok {
			retErr = errors.Wrapf(err,
				"Error evaluating the template named: '%s'", e.Name)
		}
		return "", retErr
	}
	return buffer.String(), nil
}

/*
ExtraFunctions provides additional template functions to the standard (text/template) ones,
it adds sprig functions and custom functions:

  - render - calls the render from inside of the template, making the renderer recursive
  - readFile - reads a file from a given path, relative paths are translated to absolute
          paths, based on root function
  - root - the root path for rendering, used relative to absolute path translation
          in any file based operations
  - toYaml - provides a configuration data structure fragment as a YAML format
  - gzip - use gzip compression inside the templates, for best results use with b64enc
  - ungzip - use gzip extraction inside the templates, for best results use with b64dec

*/
func (r *Renderer) ExtraFunctions() template.FuncMap {
	extraFunctions := sprig.TxtFuncMap()
	extraFunctions["render"] = r.SimpleRender
	extraFunctions["readFile"] = r.ReadFile
	extraFunctions["toYaml"] = ToYaml
	extraFunctions["ungzip"] = Ungzip
	extraFunctions["gzip"] = Gzip
	//extraFunctions["decryptAws"] = DecryptAWS
	//extraFunctions["embedDecryptAws"] = r.EmbedDecryptAws
	return extraFunctions
}
