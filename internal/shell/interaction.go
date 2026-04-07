package shell

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func PromptAction(options PromptOptions) (Action, error) {
	if options.Risk.HighRisk {
		fmt.Printf("Warning: %s\n", strings.Join(options.Risk.Reasons, "; "))
	}
	choices := "[H]ost, [S]afe, "
	if options.HasSandbox {
		choices += "[B]andbox, "
	}
	choices += "[D]escribe, [A]bort? "
	fmt.Print(choices)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return ActionAbort, err
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "h", "host":
		return ActionExecuteHost, nil
	case "s", "safe":
		return ActionExecuteSafe, nil
	case "b", "sandbox":
		if options.HasSandbox {
			return ActionExecuteSandbox, nil
		}
		return ActionAbort, nil
	case "d", "describe":
		return ActionDescribe, nil
	default:
		return ActionAbort, nil
	}
}

func ConfirmRiskyExecution(mode ExecutionMode, report RiskReport) (bool, error) {
	if !report.HighRisk {
		return true, nil
	}
	fmt.Printf("Confirm %s execution of risky command (%s)? [y/N] ", mode, strings.Join(report.Reasons, "; "))
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}
