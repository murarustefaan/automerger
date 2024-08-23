package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"time"
)

const maxAutomergePRs = 5

type label struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

type pullRequest struct {
	Id     string `json:"id"`
	Number int    `json:"number"`
	Title  string `json:"title"`
	Labels []label
}

// executeCommand runs a shell command and returns the output or an error
func executeCommand(args ...string) (string, error) {
	cmd := exec.Command("gh", args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func fetchPRsByLabel(repo string, labels []string) ([]pullRequest, error) {
	params := []string{"pr", "list", "--repo", repo, "--limit", "100"}
	for _, label := range labels {
		params = append(params, "--label", label)
	}
	params = append(params, "--json", "id,number,title,labels")

	output, err := executeCommand(params...)
	if err != nil {
		return nil, err
	}

	var prs []pullRequest
	if err = json.Unmarshal([]byte(output), &prs); err != nil {
		return nil, err
	}

	return prs, nil
}

func fetchPRs(repo string, labels []string) ([]pullRequest, error) {
	params := []string{"pr", "list", "--repo", repo, "--limit", "100"}
	for _, label := range labels {
		params = append(params, "--label", label)
	}
	params = append(params, "--json", "id,number,title,labels")

	output, err := executeCommand(params...)
	if err != nil {
		return nil, err
	}

	var prs []pullRequest
	if err = json.Unmarshal([]byte(output), &prs); err != nil {
		return nil, err
	}

	return prs, nil
}

func (pr pullRequest) Label(repo string, label string) error {
	_, err := executeCommand("pr", "edit", strconv.Itoa(pr.Number), "--repo", repo, "--add-label", label)
	return err
}

func (pr pullRequest) Delabel(repo string) error {
	_, err := executeCommand("pr", "edit", strconv.Itoa(pr.Number), "--repo", repo, "--remove-label", "automerge")
	return err
}

// main function to drive the process
func main() {
	if len(os.Args) != 2 {
		log.Fatalf("Usage: %s <repo>", os.Args[0])
	}

	repo := os.Args[1]

	for {
		resp, err := fetchPRsByLabel(repo, []string{"autorelease: pending"})
		if err != nil {
			log.Fatalf("Failed to get pending PRs: %v", err)
		}
		var pendingPRs = make([]pullRequest, 0, len(resp))
		for _, pr := range resp {
			// do not release any Framework PRs
			if !strings.HasPrefix(pr.Title, "chore(main): Release plugins-") {
				continue
			}
			// do not release any PRs that have the following labels
			if slices.ContainsFunc(pr.Labels, func(label label) bool {
				return slices.Contains([]string{"wip", "automerge", "no automerge"}, label.Name)
			}) {
				continue
			}
			pendingPRs = append(pendingPRs, pr)
		}

		if len(pendingPRs) == 0 {
			fmt.Println("No more pending PRs.")
			break
		}

		fmt.Printf("Found %d pending PRs:\n", len(pendingPRs))
		for _, pr := range pendingPRs {
			fmt.Printf("\t - #%d: %s\n", pr.Number, pr.Title)
		}
		fmt.Println()

		automergePRs, err := fetchPRsByLabel(repo, []string{"autorelease: pending", "automerge"})
		if err != nil {
			log.Fatalf("Failed to get automerge PRs: %v", err)
		}
		fmt.Printf("Found %d PRs in automerge:\n", len(automergePRs))
		for _, pr := range automergePRs {
			fmt.Printf("\t - #%d: %s\n", pr.Number, pr.Title)
		}
		fmt.Println()

		//for _, pr := range automergePRs {
		//	fmt.Printf("Removing label from PR #%d: %s\n", pr.Number, pr.Title)
		//	err := pr.Delabel(repo)
		//	if err != nil {
		//		log.Fatalf("Failed to remove label from PR #%d: %s = %v", pr.Number, pr.Title, err)
		//	}
		//}

		toLabel := maxAutomergePRs - len(automergePRs)
		if toLabel > 0 {
			toLabel = min(toLabel, len(pendingPRs))
			for i := 0; i < toLabel; i++ {
				log.Printf("Labeling PR #%s: %s with automerge.\n", strconv.Itoa(pendingPRs[i].Number), pendingPRs[i].Title)
				err := pendingPRs[i].Label(repo, "automerge")

				if err != nil {
					log.Fatalf("Failed to label PR #%s: %s = %v", pendingPRs[i].Number, pendingPRs[i].Title, err)
				}
				fmt.Printf("Labeled PR #%s: %s with automerge.\n", strconv.Itoa(pendingPRs[i].Number), pendingPRs[i].Title)
			}
		}

		fmt.Println("Waiting for merges.")
		time.Sleep(5 * time.Minute) // Adjust the sleep time as needed
	}
}
