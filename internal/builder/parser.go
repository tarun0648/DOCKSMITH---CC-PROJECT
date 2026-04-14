package builder

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// InstructionType identifies which instruction a line represents.
type InstructionType string

const (
	InstrFROM    InstructionType = "FROM"
	InstrCOPY    InstructionType = "COPY"
	InstrRUN     InstructionType = "RUN"
	InstrWORKDIR InstructionType = "WORKDIR"
	InstrENV     InstructionType = "ENV"
	InstrCMD     InstructionType = "CMD"
)

var validInstructions = map[InstructionType]bool{
	InstrFROM:    true,
	InstrCOPY:    true,
	InstrRUN:     true,
	InstrWORKDIR: true,
	InstrENV:     true,
	InstrCMD:     true,
}

// Instruction is a parsed line from a Docksmithfile.
type Instruction struct {
	Type    InstructionType
	Args    string // raw args after the instruction keyword
	LineNum int
}

// ParseFile reads and parses a Docksmithfile.
func ParseFile(path string) ([]Instruction, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("could not open Docksmithfile: %w", err)
	}
	defer f.Close()

	var instructions []Instruction
	scanner := bufio.NewScanner(f)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, " ", 2)
		keyword := InstructionType(strings.ToUpper(parts[0]))

		if !validInstructions[keyword] {
			return nil, fmt.Errorf("line %d: unrecognised instruction %q", lineNum, parts[0])
		}

		args := ""
		if len(parts) == 2 {
			args = strings.TrimSpace(parts[1])
		}

		instructions = append(instructions, Instruction{
			Type:    keyword,
			Args:    args,
			LineNum: lineNum,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return instructions, nil
}

// ParseCMD parses the JSON array form of CMD.
func ParseCMD(args string) ([]string, error) {
	var cmd []string
	if err := json.Unmarshal([]byte(args), &cmd); err != nil {
		return nil, fmt.Errorf("CMD must be a JSON array, e.g. [\"exec\",\"arg\"]: %w", err)
	}
	return cmd, nil
}

// ParseENV parses KEY=VALUE
func ParseENV(args string) (string, string, error) {
	idx := strings.IndexByte(args, '=')
	if idx < 0 {
		return "", "", fmt.Errorf("ENV requires KEY=VALUE format, got %q", args)
	}
	return args[:idx], args[idx+1:], nil
}

// ParseCOPY parses "src dest"
func ParseCOPY(args string) (src, dest string, err error) {
	parts := strings.Fields(args)
	if len(parts) < 2 {
		return "", "", fmt.Errorf("COPY requires <src> <dest>")
	}
	// Everything but last is src, last is dest
	return strings.Join(parts[:len(parts)-1], " "), parts[len(parts)-1], nil
}
