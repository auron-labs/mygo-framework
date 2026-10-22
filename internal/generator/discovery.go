// Package generator discovers and generates framework-owned adapters.
package generator

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/tools/go/packages"
)

const expectedSignature = "(context.Context, NamedInputStruct) (NamedOutputStruct, error)"

// DiscoveryConfig selects packages relative to a module directory.
type DiscoveryConfig struct {
	Dir      string
	Patterns []string
}

// NamedType identifies a concrete named struct without retaining go/types state.
type NamedType struct {
	PackagePath string
	Name        string
}

// MCP describes an explicitly exposed MCP tool and its effect hints.
type MCP struct {
	Name        string
	ReadOnly    bool
	Destructive bool
}

// Method is one supported, explicitly exposed service method.
type Method struct {
	PackagePath string
	Service     string
	Name        string
	Description string
	Command     string
	MCP         *MCP
	Input       NamedType
	Output      NamedType
	Position    token.Position
}

type annotations struct {
	command string
	mcp     *MCP
}

// Discover loads and validates explicitly annotated service methods.
func Discover(ctx context.Context, config DiscoveryConfig) ([]Method, error) {
	if ctx == nil {
		return nil, errors.New("discover service methods: context is required")
	}
	patterns := config.Patterns
	if len(patterns) == 0 {
		patterns = []string{"./app/services"}
	}
	loaded, err := packages.Load(&packages.Config{
		Context: ctx,
		Dir:     config.Dir,
		Mode:    packages.LoadSyntax,
	}, patterns...)
	if err != nil {
		return nil, fmt.Errorf("load service packages: %w", err)
	}
	if err := packageErrors(loaded); err != nil {
		return nil, err
	}
	if len(loaded) == 0 {
		return nil, errors.New("discover service methods: no packages matched")
	}

	var methods []Method
	for _, pkg := range loaded {
		for _, file := range pkg.Syntax {
			for _, declaration := range file.Decls {
				function, ok := declaration.(*ast.FuncDecl)
				if !ok || function.Recv == nil || function.Doc == nil {
					continue
				}
				position := pkg.Fset.Position(function.Pos())
				annotations, description, marked, err := parseAnnotations(function.Doc)
				if err != nil {
					return nil, diagnostic(position, err)
				}
				if !marked {
					continue
				}
				method, err := inspectMethod(pkg, function, annotations, description, position)
				if err != nil {
					return nil, err
				}
				methods = append(methods, method)
			}
		}
	}

	slices.SortFunc(methods, func(left, right Method) int {
		return strings.Compare(
			left.PackagePath+"\x00"+left.Service+"\x00"+left.Name,
			right.PackagePath+"\x00"+right.Service+"\x00"+right.Name,
		)
	})
	if err := rejectDuplicates(methods); err != nil {
		return nil, err
	}
	return methods, nil
}

func inspectMethod(
	pkg *packages.Package,
	declaration *ast.FuncDecl,
	annotations annotations,
	description string,
	position token.Position,
) (Method, error) {
	if description == "" {
		return Method{}, diagnostic(position, errors.New("marked method requires a description"))
	}
	if !declaration.Name.IsExported() {
		return Method{}, diagnostic(position, errors.New("marked method must be exported"))
	}
	object, ok := pkg.TypesInfo.Defs[declaration.Name].(*types.Func)
	if !ok {
		return Method{}, diagnostic(position, errors.New("cannot resolve marked method type"))
	}
	signature, ok := object.Type().(*types.Signature)
	if !ok || signature.Variadic() || signature.TypeParams() != nil && signature.TypeParams().Len() != 0 ||
		signature.RecvTypeParams() != nil && signature.RecvTypeParams().Len() != 0 ||
		signature.Params().Len() != 2 || signature.Results().Len() != 2 ||
		!isNamed(signature.Params().At(0).Type(), "context", "Context") ||
		!types.Identical(signature.Results().At(1).Type(), types.Universe.Lookup("error").Type()) {
		return Method{}, unsupportedSignature(position, declaration.Name.Name)
	}

	service, ok := receiverName(signature.Recv().Type())
	if !ok {
		return Method{}, unsupportedSignature(position, declaration.Name.Name)
	}
	input, ok := namedStruct(signature.Params().At(1).Type())
	if !ok {
		return Method{}, unsupportedSignature(position, declaration.Name.Name)
	}
	output, ok := namedStruct(signature.Results().At(0).Type())
	if !ok {
		return Method{}, unsupportedSignature(position, declaration.Name.Name)
	}

	return Method{
		PackagePath: pkg.PkgPath,
		Service:     service,
		Name:        declaration.Name.Name,
		Description: description,
		Command:     annotations.command,
		MCP:         annotations.mcp,
		Input:       input,
		Output:      output,
		Position:    position,
	}, nil
}

func parseAnnotations(group *ast.CommentGroup) (annotations, string, bool, error) {
	var parsed annotations
	var description []string
	marked := false
	for _, line := range commentLines(group) {
		switch {
		case strings.HasPrefix(line, "framework:command"):
			marked = true
			if parsed.command != "" {
				return annotations{}, "", true, errors.New("duplicate framework:command annotation")
			}
			fields := strings.Fields(line)
			if len(fields) != 2 || fields[0] != "framework:command" || !validName(fields[1], true) {
				return annotations{}, "", true, errors.New("framework:command requires one dotted command name")
			}
			parsed.command = fields[1]
		case strings.HasPrefix(line, "framework:mcp"):
			marked = true
			if parsed.mcp != nil {
				return annotations{}, "", true, errors.New("duplicate framework:mcp annotation")
			}
			mcp, err := parseMCP(line)
			if err != nil {
				return annotations{}, "", true, err
			}
			parsed.mcp = mcp
		case strings.HasPrefix(line, "framework:"):
			return annotations{}, "", true, fmt.Errorf("unsupported annotation %q", line)
		default:
			description = append(description, line)
		}
	}
	return parsed, strings.TrimSpace(strings.Join(description, "\n")), marked, nil
}

func parseMCP(line string) (*MCP, error) {
	fields := strings.Fields(line)
	if len(fields) != 4 || fields[0] != "framework:mcp" || !validName(fields[1], false) {
		return nil, errors.New("framework:mcp requires a tool name and explicit readonly/destructive booleans")
	}
	values := map[string]bool{}
	seen := map[string]bool{}
	for _, field := range fields[2:] {
		key, value, ok := strings.Cut(field, "=")
		if !ok || (key != "readonly" && key != "destructive") || seen[key] || (value != "true" && value != "false") {
			return nil, errors.New("framework:mcp requires unique readonly=false and destructive=false fields")
		}
		values[key], _ = strconv.ParseBool(value)
		seen[key] = true
	}
	if !seen["readonly"] || !seen["destructive"] {
		return nil, errors.New("framework:mcp requires both readonly and destructive fields")
	}
	if values["readonly"] && values["destructive"] {
		return nil, errors.New("framework:mcp cannot be both readonly and destructive")
	}
	return &MCP{Name: fields[1], ReadOnly: values["readonly"], Destructive: values["destructive"]}, nil
}

func commentLines(group *ast.CommentGroup) []string {
	var lines []string
	for _, comment := range group.List {
		text := comment.Text
		if strings.HasPrefix(text, "//") {
			lines = append(lines, strings.TrimSpace(strings.TrimPrefix(text, "//")))
			continue
		}
		text = strings.TrimSuffix(strings.TrimPrefix(text, "/*"), "*/")
		for line := range strings.SplitSeq(text, "\n") {
			lines = append(lines, strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "*")))
		}
	}
	return lines
}

func validName(value string, dotted bool) bool {
	parts := []string{value}
	if dotted {
		parts = strings.Split(value, ".")
	}
	for _, part := range parts {
		if part == "" {
			return false
		}
		for index, char := range part {
			if unicode.IsLower(char) || unicode.IsDigit(char) || char == '_' || char == '-' {
				if index == 0 && !unicode.IsLower(char) {
					return false
				}
				continue
			}
			return false
		}
	}
	return true
}

func receiverName(value types.Type) (string, bool) {
	if pointer, ok := value.(*types.Pointer); ok {
		value = pointer.Elem()
	}
	named, ok := value.(*types.Named)
	if !ok {
		return "", false
	}
	_, ok = named.Underlying().(*types.Struct)
	return named.Obj().Name(), ok
}

func namedStruct(value types.Type) (NamedType, bool) {
	named, ok := value.(*types.Named)
	if !ok {
		return NamedType{}, false
	}
	if _, ok := named.Underlying().(*types.Struct); !ok || named.Obj().Pkg() == nil {
		return NamedType{}, false
	}
	return NamedType{PackagePath: named.Obj().Pkg().Path(), Name: named.Obj().Name()}, true
}

func isNamed(value types.Type, packagePath, name string) bool {
	named, ok := value.(*types.Named)
	return ok && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == packagePath && named.Obj().Name() == name
}

func rejectDuplicates(methods []Method) error {
	commands := map[string]Method{}
	tools := map[string]Method{}
	for _, method := range methods {
		if method.Command != "" {
			if previous, exists := commands[method.Command]; exists {
				return diagnostic(method.Position, fmt.Errorf("duplicate command %q; also declared at %s", method.Command, previous.Position))
			}
			commands[method.Command] = method
		}
		if method.MCP != nil {
			if previous, exists := tools[method.MCP.Name]; exists {
				return diagnostic(method.Position, fmt.Errorf("duplicate MCP tool %q; also declared at %s", method.MCP.Name, previous.Position))
			}
			tools[method.MCP.Name] = method
		}
	}
	return nil
}

func packageErrors(loaded []*packages.Package) error {
	var messages []string
	for _, pkg := range loaded {
		for _, err := range pkg.Errors {
			messages = append(messages, err.Error())
		}
	}
	if len(messages) == 0 {
		return nil
	}
	sort.Strings(messages)
	return fmt.Errorf("load service packages:\n%s", strings.Join(messages, "\n"))
}

func unsupportedSignature(position token.Position, method string) error {
	return diagnostic(position, fmt.Errorf("marked method %s must have signature %s", method, expectedSignature))
}

func diagnostic(position token.Position, err error) error {
	return fmt.Errorf("%s: %w", position, err)
}
