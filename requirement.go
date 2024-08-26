package main

import (
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/curtisnewbie/miso/util"
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

const (
	codeBlock = "```"
)

const (
	EnvRequirementFile = "REQUIREMENTS_FILE"
)

var (
	titleRegex  = regexp.MustCompile(`(?i)^- \[([\* Xx]*)\] *(.*)`)
	docRegex    = regexp.MustCompile(`(?i)^ {2}- (文档|需求|doc|docs|documents|documentation|documentations):\s*`)
	reposRegex  = regexp.MustCompile(`(?i)^ {2}- (服务|代码仓库|服务列表|服务|service|services|repo|repos|repository|repositories):\s*`)
	branchRegex = regexp.MustCompile(`(?i)^ {2}- (分支|branch|branches):\s*`)
	todoRegex   = regexp.MustCompile(`(?i)^ {2}- (待办|todo|todos):\s*`)
	bulletRegex = regexp.MustCompile(`^ {4}- *(.*)`)
	tildeRegex  = regexp.MustCompile(`(~~.*~~)`)
)

var (
	All  = flag.Bool("all", false, "Show all active requirements")
	New  = flag.Bool("new", false, "Add new requirement")
	Name = flag.String("name", "", "New requirement name")
	// Archive = flag.String("archive", "", "Archive requirement by name")
)

func main() {
	flag.Parse()

	branch, cwd := ParseCurrRepo()
	file := os.Getenv(EnvRequirementFile)

	if util.IsBlankStr(file) {
		util.Printlnf("Where is the requirement file?")
		return
	}

	if *New {
		af, err := util.AppendableFile(file)
		util.Must(err)
		defer af.Close()
		name := *Name
		if util.IsBlankStr(name) {
			name = "TODO"
		}
		af.WriteString(util.NamedSprintf(`
- [ ] ${name}
  - 文档:
    - 记录时间: ${today}
    - 需求地址:
    - 技术文档:
    - 需求文档:
    - 发布单:
    - UI:
  - 服务:
    - ${repo}
  - 分支:
    - ${branch}
  - 待办:
    - [ ]
  `, map[string]any{
			"name":   name,
			"today":  util.Now().FormatDate(),
			"repo":   cwd,
			"branch": branch,
		}))
		return
	}

	byt, err := util.ReadFileAll(file)
	util.Must(err)

	content := util.UnsafeByt2Str(byt)
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
	requirements := util.NewStack[*Requirement](30)

	for i := 0; i < len(splited); i++ {
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

			curr, ok := requirements.Peek()
			if !ok {
				panic(fmt.Errorf("illegal format, line: %v", l))
			}
			curr.AddCodeBlock(l)
			i = j + 1
			continue
		}

		if matched := titleRegex.FindStringSubmatch(l); len(matched) > 0 {
			curr := NewRequirement(matched[2])
			if strings.TrimSpace(matched[1]) != "" {
				curr.Done = true
			}
			requirements.Push(curr)
		} else {
			curr, ok := requirements.Peek()
			if !ok {
				panic(fmt.Errorf("illegal format, line: %v", l))
			}

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
		}
	}

	isMain := branch == "master" || branch == "main"
	filtered := util.Filter(requirements.Slice(), func(r *Requirement) bool { return !r.Done })

	if *All || (branch == "") {
		util.Printlnf("Found %v%v%v Requirements\n", red, len(filtered), reset)
		for _, req := range filtered {
			util.Printlnf("%v", req)
		}
	} else {
		mr := make([]*Requirement, 0, len(filtered))
		for _, req := range filtered {
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

		util.Printlnf("Found %v%v%v Requirements, Matched %v%v%v Requirements\n",
			red, len(filtered), reset, red, len(mr), reset)
		for _, req := range mr {
			util.Printlnf(req.String())
		}
	}
}

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

func (r *Requirement) AddCodeBlock(s string) {
	s = strings.TrimSpace(s)
	i := strings.Index(s, "\n")
	if i > -1 {
		s = s[i+1:]
	}
	s, _ = strings.CutSuffix(s, codeBlock)
	sp := strings.Split(s, "\n")
	s = "  " + strings.Join(sp, "\n  ")

	r.CodeBlocks = append(r.CodeBlocks, s)
}

var bracketPat = regexp.MustCompile(`[【】]`)

func (r *Requirement) SetName(s string) {
	r.Name = bracketPat.ReplaceAllStringFunc(s, func(s string) string {
		switch s {
		case "【":
			return "["
		case "】":
			return "] "
		default:
			return s
		}
	})
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

func NewRequirement(name string) *Requirement {
	r := Requirement{}
	r.SetName(name)
	r.Docs = []string{}
	r.Repos = []string{}
	r.Branches = []string{}
	r.Todos = []string{}
	r.BranchMatched = -1
	r.RepoMatched = -1
	r.CodeBlocks = []string{}
	return &r
}

func ParseTilde(v string) string {
	return tildeRegex.ReplaceAllStringFunc(v, func(s string) string {
		if len(s) < 1 {
			return s
		}
		return dim + s[2:len(s)-2] + reset
	})
}

func ParseCurrRepo() (branch string, cwd string) {
	cmdout, err := util.CliRun("git", "status")
	if err != nil {
		util.Printlnf("%sNot a git repository, %v%s", red, err, reset)
	} else {
		outstr := util.UnsafeByt2Str(cmdout)
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
		util.Printlnf("On Branch %v%v%v", red, branch, reset)

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
		util.Printlnf("Current Repo: %v%s%v", red, cwd, reset)
	}
	return
}
