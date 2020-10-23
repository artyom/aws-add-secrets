// Command aws-add-secrets loads secrets from a CSV file to an AWS Secrets
// Manager.
//
// CSV file must have a header, which is inspected to find "name", "value", and
// an optional "description" columns.
//
// It outputs ARNs of each secret created, or a JSON lines suitable for the
// "secrets" section of ECS container task definition if run with an -env flag.
package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/artyom/csvstruct"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
)

func main() {
	log.SetFlags(0)
	envJson := flag.Bool("env", false, "output json record for each secret created instead of ARN (for ECS task definition)")
	flag.Parse()
	if err := run(flag.Arg(0), *envJson); err != nil {
		log.Fatal(err)
	}
}

func run(file string, envJson bool) error {
	if file == "" {
		return errors.New("input file missing")
	}
	secrets, err := readSecrets(file)
	if err != nil {
		return err
	}
	if len(secrets) == 0 {
		return errors.New("file has no secrets")
	}
	sess, err := session.NewSession()
	if err != nil {
		return err
	}
	ctx := context.Background()
	svc := secretsmanager.New(sess)
	for _, s := range secrets {
		out, err := svc.CreateSecretWithContext(ctx, &secretsmanager.CreateSecretInput{
			Name:         &s.Name,
			SecretString: &s.Value,
			Description:  &s.Description,
		})
		if err != nil {
			return fmt.Errorf("create secret %q: %w", s.Name, err)
		}
		if envJson {
			fmt.Println(toJson(s.Name, *out.ARN))
		} else {
			fmt.Println(*out.ARN)
		}
	}
	return nil
}

type secret struct {
	Name        string `csv:"name"`
	Value       string `csv:"value"`
	Description string `csv:"description"`
}

func (s *secret) validate() error {
	if s.Name == "" {
		return errors.New("empty secret name")
	}
	if s.Value == "" {
		return errors.New("empty secret value")
	}
	return nil
}

func readSecrets(name string) ([]secret, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.ReuseRecord = true
	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("csv header read: %w", err)
	}
	scan, err := csvstruct.NewScanner(header, &secret{})
	if err != nil {
		return nil, err
	}
	var out []secret
	for {
		row, err := r.Read()
		if err != nil {
			if err == io.EOF {
				return out, nil
			}
			return nil, err
		}
		var s secret
		if err := scan(row, &s); err != nil {
			return nil, err
		}
		if err := s.validate(); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
}

// toJson returns json value that can be used as a "secrets" array element of
// an ECS task definition. It derives variable name from the secret name.
func toJson(name, arn string) string {
	if i := strings.LastIndexByte(name, '/'); i != -1 {
		name = name[i+1:]
	}
	name = strings.Map(func(r rune) rune {
		switch r {
		case ' ', '-', '_':
			return '_'
		}
		if r >= 'A' && r <= 'Z' {
			return r
		}
		return -1
	}, strings.ToUpper(name))
	b, err := json.Marshal(struct {
		Name  string `json:"name"`
		Value string `json:"valueFrom"`
	}{Name: name, Value: arn})
	if err != nil {
		panic(err)
	}
	return string(b)
}

func init() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [flags] path/to/file.csv\n", filepath.Base(os.Args[0]))
		flag.PrintDefaults()
		fmt.Fprintln(flag.CommandLine.Output(),
			"\ncsv file must have a header, inspected fields are: "+
				"'name', 'value', and 'description' (optional)")
	}
}
