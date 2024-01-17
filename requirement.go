package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

const (
	// https://stackoverflow.com/questions/8357203/is-it-possible-to-display-text-in-a-console-with-a-strike-through-effect
	dim           = "\033[2m"
	strikethrough = "\033[9m"
	red           = "\033[1;31m"
	reset         = "\033[0m"
	green         = "\033[1;32m"
	cyan          = "\033[1;36m"
	redBlink      = "\033[1;5;31m"
)

var (
	titleRegex  = regexp.MustCompile(`(?i)^- \[[\* Xx]*\] *(.*)`)
	docRegex    = regexp.MustCompile(`(?i)^ {2}- (文档|需求|doc|docs|documents|documentation|documentations):\s*`)
	reposRegex  = regexp.MustCompile(`(?i)^ {2}- (服务|代码仓库|服务列表|服务|service|services|repo|repos|repository|repositories):\s*`)
	branchRegex = regexp.MustCompile(`(?i)^ {2}- (分支|branch|branches):\s*`)
	todoRegex   = regexp.MustCompile(`(?i)^ {2}- (待办|todo|todos):\s*`)
	bulletRegex = regexp.MustCompile(`^ {4}- *(.*)`)
	tildeRegex  = regexp.MustCompile(`(~~.*~~)`)
)

type Requirement struct {
	Name     string
	Docs     []string
	Repos    []string
	Branches []string
	Todos    []string

	InDoc      bool
	InRepos    bool
	InTodos    bool
	InBranches bool

	BranchMatched int
	RepoMatched   int
}

func (r *Requirement) ResetFlags() {
	r.InRepos = false
	r.InDoc = false
	r.InBranches = false
	r.InTodos = false
}

func (r Requirement) String() string {
	s := ""
	s += fmt.Sprintf("%v%v%v\n", green, r.Name, reset)
	if len(r.Docs) > 0 {
		copied := make([]string, len(r.Docs))
		for i, v := range r.Docs {
			copied[i] = ParseTilde(v)
		}
		s += fmt.Sprintf(" Docs: \n    %+v\n", strings.Join(copied, "\n    "))
	}
	if len(r.Repos) > 0 {
		joined := ""
		for i, b := range r.Repos {
			// if i == r.RepoMatched {
			// 	joined += redBlink + b + reset
			// } else {
			// joined += ParseTilde(b)
			// }
			joined += ParseTilde(b)
			if i < len(r.Repos)-1 {
				joined += "\n    "
			}
		}
		s += fmt.Sprintf(" Repos: \n    %v\n", joined)
	}
	if len(r.Branches) > 0 {
		joined := ""
		for i, b := range r.Branches {
			if i == r.BranchMatched {
				joined += redBlink + b + reset
			} else {
				joined += ParseTilde(b)
			}
			if i < len(r.Branches)-1 {
				joined += "\n    "
			}
		}
		s += fmt.Sprintf(" Branches: \n    %v\n", joined)
	}
	if len(r.Todos) > 0 {
		copied := make([]string, len(r.Todos))
		for i, v := range r.Todos {
			if strings.HasPrefix(v, "[x]") {
				copied[i] = dim + v + reset
			} else {
				copied[i] = ParseTilde(v)
			}
		}
		s += fmt.Sprintf(" Todos: \n    %+v\n", strings.Join(copied, "\n    "))
	}
	// s += fmt.Sprintf(" Flags: InDoc:%v, InRepos: %v, InTodos: %v, InBranches: %v\n", r.InDoc, r.InRepos, r.InTodos, r.InBranches)
	return s
}

func NewRequirement(name string) Requirement {
	r := Requirement{}
	r.Docs = []string{}
	r.Repos = []string{}
	r.Branches = []string{}
	r.Todos = []string{}
	r.Name = name
	r.BranchMatched = -1
	r.RepoMatched = -1
	return r
}

func ParseTilde(v string) string {
	return tildeRegex.ReplaceAllStringFunc(v, func(s string) string {
		if len(s) < 1 {
			return s
		}
		return dim + s[2:len(s)-2] + reset
	})
}

func SearchRequirements() {
	cmd := exec.Command("git", "status")
	var cmdout []byte
	var err error
	if cmdout, err = cmd.CombinedOutput(); err != nil {
		fmt.Printf("%sNot a git repository, %v%s", red, err, reset)
		return
	}
	outstr := string(cmdout)

	lines := strings.Split(outstr, "\n")
	if len(lines) < 1 {
		fmt.Printf("%sNot a git repository%s", red, reset)
		return
	}

	l := strings.Index(lines[0], "On branch")
	branch := ""
	if l > -1 {
		branch = string([]rune(lines[0])[l+10:])
	}
	fmt.Printf("On Branch %v%v%v\n", red, branch, reset)

	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	cwdr := []rune(cwd)
	for i := len(cwdr) - 1; i >= 0; i-- {
		if cwdr[i] == '/' {
			cwd = string(cwdr[i+1:])
			break
		}
	}
	fmt.Printf("Current Repo: %v%s%v\n", red, cwd, reset)

	reqFileName := os.Getenv("REQUIREMENTS_FILE")
	reqFile, err := os.Open(reqFileName)
	if err != nil {
		panic(err)
	}

	byt, err := io.ReadAll(reqFile)
	if err != nil {
		panic(err)
	}

	content := string(byt)

	splited := strings.Split(content, "\n")
	if len(splited) < 1 {
		fmt.Printf("%sRequirement not found%s", red, reset)
	}

	requirements := []Requirement{}

	start := 0
	for i, l := range splited {
		if strings.TrimSpace(l) == "## Active Requirements" {
			start = i
			break
		}
	}

	for i := start; i < len(splited); i++ {
		l := splited[i]

		if strings.HasPrefix(l, "#") || strings.HasPrefix(l, "```") || strings.TrimSpace(l) == "" {
			continue
		}

		if matched := titleRegex.FindStringSubmatch(l); len(matched) > 0 {
			curr := NewRequirement(matched[1])
			requirements = append(requirements, curr)
		} else {
			if len(requirements) < 1 {
				panic(fmt.Errorf("illegal format, line: %v", l))
			}
			curr := requirements[len(requirements)-1]

			if matched := docRegex.FindStringSubmatch(l); len(matched) > 0 {
				curr.ResetFlags()
				curr.InDoc = true
			} else if matched := reposRegex.FindStringSubmatch(l); len(matched) > 0 {
				curr.ResetFlags()
				curr.InRepos = true
			} else if matched := todoRegex.FindStringSubmatch(l); len(matched) > 0 {
				curr.ResetFlags()
				curr.InTodos = true
			} else if matched := branchRegex.FindStringSubmatch(l); len(matched) > 0 {
				curr.ResetFlags()
				curr.InBranches = true
			} else if matched := bulletRegex.FindStringSubmatch(l); len(matched) > 0 {
				line := matched[1]
				if strings.TrimSpace(line) != "" {
					if curr.InDoc {
						tokens := strings.Split(line, ":")
						if len(tokens) > 1 && strings.TrimSpace(strings.Join(tokens[1:], "")) != "" {
							curr.Docs = append(curr.Docs, line)
						}
					} else if curr.InRepos {
						curr.Repos = append(curr.Repos, line)
					} else if curr.InTodos {
						curr.Todos = append(curr.Todos, line)
					} else if curr.InBranches {
						curr.Branches = append(curr.Branches, line)
					}
				}
			}
			requirements[len(requirements)-1] = curr
		}
	}
	// fmt.Println(requirements)

	mr := []Requirement{}
	for _, req := range requirements {
		matched := false
		for i, v := range req.Branches {
			if branch != "master" && strings.Contains(v, branch) {
				req.BranchMatched = i
				mr = append(mr, req)
				matched = true
				break
			}
		}
		if matched {
			continue
		}
		for i, v := range req.Repos {
			if strings.Contains(v, cwd) {
				req.RepoMatched = i
				mr = append(mr, req)
				matched = true
				break
			}
		}
	}

	fmt.Printf("Found %v%v%v Requirements, Matched %v%v%v Requirements\n\n", red, len(requirements), reset, red, len(mr), reset)

	for _, req := range mr {
		fmt.Printf("%v\n", req)
	}
}
