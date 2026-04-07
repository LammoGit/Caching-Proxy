package filter

import (
    "os"
    "regexp"
    "bufio"
    "strings"
)

type Filter struct {
    regex *regexp.Regexp
    Valid []string
    Invalid []string
}

func (filter *Filter) LoadRegex(pattern string) error {
    regex, err := regexp.Compile(pattern)
    if err != nil {
        return err
    }

    filter.regex = regex
    filter.Valid = append(filter.Valid, pattern)

    return nil
}

func (filter *Filter) Load(path string) error {
    file, err := os.Open(path)
    if err != nil {
        return filter.LoadRegex(path)
    }
    defer file.Close()

    filter.regex = nil
    filter.Valid = []string{}
    filter.Invalid = []string{}

    var pattern string
    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
        pattern = scanner.Text()
        _, err = regexp.Compile(pattern)
        if err == nil {
            filter.Valid = append(filter.Valid, pattern)
        } else {
            filter.Invalid = append(filter.Invalid, pattern)
        }
    }

    pattern = strings.Join(filter.Valid, "|")
    filter.regex = regexp.MustCompile(pattern)

    return nil
}

func (filter *Filter) Match(text string) bool {
    if filter.regex == nil {
        return false
    }

    return filter.regex.Match([]byte(text))
}
