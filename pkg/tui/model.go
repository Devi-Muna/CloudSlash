package tui

import (
	"time"

	"github.com/DrSkyle/cloudslash/pkg/graph"
	"github.com/DrSkyle/cloudslash/pkg/engine/swarm"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

type ViewState int

const (
	ViewStateList ViewState = iota
	ViewStateDetail
	ViewStateTopology // Added: Hierarchy View
	ViewStateHelp
)

type TopologyLine struct {
	ID    string
	Text  string
	Level int
	Node  *graph.Node
}

type Model struct {
	// core components
	spinner  spinner.Model
	progress progress.Model
	Engine   *swarm.Engine
	Graph   *graph.Graph

	// state
	state      ViewState
	scanning   bool
	quitting   bool
	err        error
	width      int
	height     int
	isMock     bool
	Region     string

	// data
	wasteItems   []*graph.Node
	topologyLines []TopologyLine // Added: Flattened tree for display
	totalSavings float64
	riskScore    int
	tasksDone    int
	tfRepairReady bool
	
	// metrics
	startTime time.Time

	// filters
	SortMode   string
	FilterMode string
	
	// feedback
	statusMsg  string
	statusTime time.Time

	// navigation
	cursor       int // main list cursor
	topologyCursor int // topology view cursor
	detailsScroll int

	// animation
	tickCount int
}

type tickMsg time.Time

func NewModel(e *swarm.Engine, g *graph.Graph, isMock bool, region string) Model {
	s := spinner.New()
	s.Spinner = spinner.Points // "Future" style spinner (dots)
	s.Style = special

	// Gradient Progress Bar (Green to Cyan)
	prog := progress.New(progress.WithGradient("#00FF99", "#00CCFF"))

	return Model{
		spinner:  s,
		progress: prog,
		scanning: !isMock,
		isMock:   isMock,
		Engine:   e,
		Graph:    g,
		state:    ViewStateList,
		startTime: time.Now(),
		Region:    region,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
			return tickMsg(t)
		}),
	)
}
