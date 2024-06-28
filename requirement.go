package requirement

import (
	"flag"
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
	titleRegex  = regexp.MustCompile(`(?i)^- \[([\* Xx]*)\] *(.*)`)
	docRegex    = regexp.MustCompile(`(?i)^ {2}- (文档|需求|doc|docs|documents|documentation|documentations):\s*`)
	reposRegex  = regexp.MustCompile(`(?i)^ {2}- (服务|代码仓库|服务列表|服务|service|services|repo|repos|repository|repositories):\s*`)
	branchRegex = regexp.MustCompile(`(?i)^ {2}- (分支|branch|branches):\s*`)
	todoRegex   = regexp.MustCompile(`(?i)^ {2}- (待办|todo|todos):\s*`)
	bulletRegex = regexp.MustCompile(`^ {4}- *(.*)`)
	tildeRegex  = regexp.MustCompile(`(~~.*~~)`)

	flagAll = flag.Bool("all", false, "Show all active requirements")
)

type Requirement struct {
	Done       bool
	Name       string
	Docs       []string
	Repos      []string
	Branches   []string
	Todos      []string
	CodeBlocks []string

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

func (r Requirement) AddCodeBlock(s string) Requirement {
	s = strings.TrimSpace(s)
	i := strings.Index(s, "\n")
	if i > -1 {
		s = s[i+1:]
	}
	s, _ = strings.CutSuffix(s, "```")
	sp := strings.Split(s, "\n")
	s = "  " + strings.Join(sp, "\n  ")

	r.CodeBlocks = append(r.CodeBlocks, s)
	return r
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
			if r.RepoMatched > -1 && i == r.BranchMatched {
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
	if len(r.CodeBlocks) > 0 {
		copied := make([]string, len(r.CodeBlocks))
		for i, v := range r.CodeBlocks {
			copied[i] = cyan + v + reset
		}
		s += fmt.Sprintf(" Code Blocks: \n\n%+v\n", strings.Join(copied, "\n\n"))
	}
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
	r.CodeBlocks = []string{}
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

func ParseRequirements() {
	flag.Parse()
	var branch string
	var cwd string

	cmd := exec.Command("git", "status")
	var cmdout []byte
	var err error
	if cmdout, err = cmd.CombinedOutput(); err != nil {
		fmt.Printf("%sNot a git repository, %v%s\n", red, err, reset)
	} else {
		outstr := UnsafeByt2Str(cmdout)
		lines := strings.Split(outstr, "\n")
		if len(lines) < 1 {
			fmt.Printf("%sNot a git repository%s", red, reset)
			return
		} else {
			l := strings.Index(lines[0], "On branch")
			branch = ""
			if l > -1 {
				branch = string([]rune(lines[0])[l+10:])
			}
		}
		fmt.Printf("On Branch %v%v%v\n", red, branch, reset)

		cw, err := os.Getwd()
		if err != nil {
			panic(err)
		}
		cwd = cw
		cwdr := []rune(cw)
		for i := len(cwdr) - 1; i >= 0; i-- {
			if cwdr[i] == '/' {
				cwd = string(cwdr[i+1:])
				break
			}
		}
		fmt.Printf("Current Repo: %v%s%v\n", red, cwd, reset)
	}

	reqFileName := os.Getenv("REQUIREMENTS_FILE")
	reqFile, err := os.Open(reqFileName)
	if err != nil {
		panic(err)
	}

	byt, err := io.ReadAll(reqFile)
	if err != nil {
		panic(err)
	}

	content := UnsafeByt2Str(byt)
	i := strings.Index(content, "## Active Requirements")
	if i > 0 {
		for j := i; j < len(content); j++ {
			if content[j] == '\n' {
				if j+1 < len(content) {
					j = j + 1
				}
				content = content[j+1:]
				break
			}
		}
	}

	splited := strings.Split(content, "\n")
	if len(splited) < 1 {
		fmt.Printf("%sRequirement not found%s", red, reset)
	}
	requirements := make([]Requirement, 0, 30)
	codeBlock := "```"

	start := 0

	for i := start; i < len(splited); i++ {
		l := splited[i]

		if strings.HasPrefix(l, "#") || strings.TrimSpace(l) == "" {
			continue
		}
		if strings.HasPrefix(l, codeBlock) {
			j := i + 1
			for ; j < len(splited) && !strings.HasPrefix(splited[j], codeBlock); j++ {
			}
			if j == i || j >= len(splited) {
				continue
			}
			l = strings.Join(splited[i:j+1], "\n")

			if len(requirements) < 1 {
				panic(fmt.Errorf("illegal format, line: %v", l))
			}
			requirements[len(requirements)-1] = requirements[len(requirements)-1].AddCodeBlock(l)
			i = j + 1
			continue
		}

		if matched := titleRegex.FindStringSubmatch(l); len(matched) > 0 {
			curr := NewRequirement(matched[2])
			if strings.TrimSpace(matched[1]) != "" {
				curr.Done = true
			}
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

	isMain := branch == "master" || branch == "main"
	requirements = Filter(requirements, func(r Requirement) bool { return !r.Done })

	if *flagAll || (branch == "") {
		fmt.Printf("Found %v%v%v Requirements\n\n", red, len(requirements), reset)
		for _, req := range requirements {
			fmt.Printf("%v\n", req)
		}
	} else {
		mr := []Requirement{}
		for _, req := range requirements {
			matched := false
			for i, v := range req.Repos {
				if strings.Contains(v, cwd) {
					req.RepoMatched = i
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
			for i, v := range req.Branches {
				if !isMain && strings.Contains(v, branch) {
					req.BranchMatched = i
					break
				}
			}
			mr = append(mr, req)
		}

		fmt.Printf("Found %v%v%v Requirements, Matched %v%v%v Requirements\n\n", red, len(requirements), reset, red, len(mr), reset)
		for _, req := range mr {
			fmt.Printf("%v\n", req)
		}
	}

}

func Filter[T any](l []T, f func(T) bool) []T {
	cp := l[:0]
	for i := range l {
		x := l[i]
		if f(x) {
			cp = append(cp, x)
		}
	}
	for i := len(cp); i < len(l); i++ {
		var nt T
		l[i] = nt
	}
	return cp
}
