package commands

import (
	"fmt"
	"strings"

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
		choices:  []string{"us-east-1", "us-east-2", "us-west-1", "us-west-2", "eu-central-1", "eu-west-1", "ap-southeast-1", "ap-northeast-1"},
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
			}
		case "down", "j":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
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

	for i, choice := range m.choices {
		cursor := " "
		if m.cursor == i {
			cursor = ">"
		}

		checked := " "
		if _, ok := m.selected[i]; ok {
			checked = "x"
		}

		s.WriteString(fmt.Sprintf("%s [%s] %s\n", cursor, checked, choice))
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
