package filter

import (
    "os"
    "regexp"
    "bufio"
    "fmt"
    "strings"
)

type Filter struct {
    WhiteRegex     *regexp.Regexp
    BlackRegex     *regexp.Regexp
    WhitePatterns  []string
    BlackPatterns  []string
}

func (filter *Filter) Load(whitePath, blackPath string) error {
    wlFile, err := os.Open(whitePath)
    if err != nil {
        return fmt.Errorf("Couldn't open whitelist file at path: %s\n", whitePath)
    }
    defer wlFile.Close()


    blFile, err := os.Open(blackPath)
    if err != nil {
        return fmt.Errorf("Couldn't open blacklist file at path: %s\n", blackPath)
    }
    defer blFile.Close()

    wlScanner := bufio.NewScanner(wlFile)
    blScanner := bufio.NewScanner(blFile)

    filter.WhitePatterns = make([]string, 0)
    filter.BlackPatterns = make([]string, 0)

    var line string
    for wlScanner.Scan() {
        line = wlScanner.Text()
        _, err = regexp.Compile(line)
        if err != nil {
            fmt.Printf("Whitelist pattern is invalid, therefore removed: %s\n", line)
            continue
        }
        filter.WhitePatterns = append(filter.WhitePatterns, line)
    }

    for blScanner.Scan() {
        line = blScanner.Text()
        _, err = regexp.Compile(line)
        if err != nil {
            fmt.Printf("Blacklist pattern is invalid, therefore removed: %s\n", line)
            continue
        }
        filter.BlackPatterns = append(filter.BlackPatterns, line)
    }

    fmt.Println(strings.Join(filter.WhitePatterns, "|"))
    fmt.Println(strings.Join(filter.BlackPatterns, "|"))

    if len(filter.WhitePatterns) > 0 {
        filter.WhiteRegex = regexp.MustCompile("^" + strings.Join(filter.WhitePatterns, "$|^") + "$")
    } else {
        filter.WhiteRegex = nil
    }

    if len(filter.BlackPatterns) > 0 {
        filter.BlackRegex = regexp.MustCompile("^" + strings.Join(filter.BlackPatterns, "|") + "$")
   } else {
        filter.BlackRegex = nil
   }

    return nil
}

func (filter *Filter) Match(text string) bool {

    return !(filter.BlackRegex != nil && filter.BlackRegex.MatchString(text)) &&
    (filter.WhiteRegex != nil && filter.WhiteRegex.MatchString(text))
}

func (filter *Filter) MatchAny(texts []string) int {
    if filter.WhiteRegex == nil || filter.BlackRegex == nil {
        return -1
    }

    for idx, text := range texts {
        if filter.Match(text) {
            return idx
        }
    }

    return -1
}
