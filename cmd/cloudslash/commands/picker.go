package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	internalaws "github.com/DrSkyle/cloudslash/internal/aws"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type regionModel struct {
	choices  []string
	selected map[int]struct{}
	cursor   int
}

func initialRegionModel() regionModel {
	return regionModel{
		choices: []string{
			// US
			"us-east-1", "us-east-2", "us-west-1", "us-west-2",
			// Europe
			"eu-central-1", "eu-central-2", "eu-west-1", "eu-west-2", "eu-west-3", "eu-north-1", "eu-south-1", "eu-south-2",
			// APAC
			"ap-south-1", "ap-south-2", "ap-southeast-1", "ap-southeast-2", "ap-southeast-3", "ap-southeast-4",
			"ap-northeast-1", "ap-northeast-2", "ap-northeast-3", "ap-east-1",
			// Americas
			"ca-central-1", "ca-west-1", "sa-east-1",
			// Africa / Middle East
			"af-south-1", "me-south-1", "me-central-1", "il-central-1",
		},
		selected: make(map[int]struct{}),
	}
}

func (m regionModel) Init() tea.Cmd {
	return nil
}

func (m regionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			} else {
				// Loop to bottom
				m.cursor = len(m.choices) - 1
			}
		case "down", "j":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			} else {
				// Loop to top
				m.cursor = 0
			}
		case " ", "x":
			_, ok := m.selected[m.cursor]
			if ok {
				delete(m.selected, m.cursor)
			} else {
				m.selected[m.cursor] = struct{}{}
			}
		case "enter":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m regionModel) View() string {
	s := strings.Builder{}
	s.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).Render("? Which regions do you want to scan?"))
	s.WriteString("\n\n")

	// Paginator Logic
	// Window size = 10
	windowHeight := 10
	start := 0
	end := len(m.choices)

	if len(m.choices) > windowHeight {
		if m.cursor < windowHeight/2 {
			start = 0
			end = windowHeight
		} else if m.cursor >= len(m.choices)-windowHeight/2 {
			start = len(m.choices) - windowHeight
			end = len(m.choices)
		} else {
			start = m.cursor - windowHeight/2
			end = m.cursor + windowHeight/2
		}
	}

	for i := start; i < end; i++ {
		choice := m.choices[i]
		cursor := " "
		if m.cursor == i {
			cursor = ">"
		}

		checked := " "
		if _, ok := m.selected[i]; ok {
			checked = "x"
		}

		style := lipgloss.NewStyle()
		if m.cursor == i {
			style = style.Foreground(lipgloss.Color("205")).Bold(true)
		}

		line := fmt.Sprintf("%s [%s] %s", cursor, checked, choice)
		s.WriteString(style.Render(line) + "\n")
	}
	
	if len(m.choices) > windowHeight {
		s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(fmt.Sprintf("\n... %d more regions (use arrow keys) ...\n", len(m.choices)-end)))
	}

	s.WriteString("\n(Press [space] to select, [enter] to confirm)\n")
	return s.String()
}

func (m regionModel) GetSelectedRegions() []string {
	var selected []string
	// If nothing selected, default to us-east-1? Or all?
	// User said: "Interactive Selection".
	if len(m.selected) == 0 {
		return []string{"us-east-1"}
	}
	for i := range m.selected {
		selected = append(selected, m.choices[i])
	}
	return selected
}

func PromptForRegions() ([]string, error) {
	// 1. Try Dynamic Fetch (Smart Discovery)
	dynamicRegions, err := fetchDynamicRegions()
	if err == nil && len(dynamicRegions) > 0 {
		// Use dynamic list!
		p := tea.NewProgram(regionModel {
			choices: dynamicRegions,
			selected: make(map[int]struct{}),
		})
		m, err := p.Run()
		if err != nil { return nil, err }
		if regionModel, ok := m.(regionModel); ok {
			return regionModel.GetSelectedRegions(), nil
		}
		return []string{"us-east-1"}, nil
	}

	// 2. Fallback to Static List
	p := tea.NewProgram(initialRegionModel())
	m, err := p.Run()
	if err != nil {
		return nil, err
	}

	if regionModel, ok := m.(regionModel); ok {
		return regionModel.GetSelectedRegions(), nil
	}
	return []string{"us-east-1"}, nil
}

func fetchDynamicRegions() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Initialize robust AWS client (using default profile)
	client, err := internalaws.NewClient(ctx, "us-east-1", "", false)
	if err != nil {
		return nil, err
	}

	// Create EC2 Client
	ec2Client := ec2.NewFromConfig(client.Config)
	
	// Describe Regions
	resp, err := ec2Client.DescribeRegions(ctx, &ec2.DescribeRegionsInput{
		AllRegions: aws.Bool(true), // Fetch Opt-In Regions too if enabled
	})
	if err != nil {
		return nil, err
	}

	var regions []string
	for _, r := range resp.Regions {
		if r.RegionName != nil {
			regions = append(regions, *r.RegionName)
		}
	}
	return regions, nil
}
