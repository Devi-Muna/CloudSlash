package ui

import (
	"time"

	"github.com/DrSkyle/cloudslash/internal/graph"
	"github.com/DrSkyle/cloudslash/internal/swarm"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

type ViewState int

const (
	ViewStateList ViewState = iota
	ViewStateDetail
	ViewStateHelp
)

type Model struct {
	// core components
	spinner spinner.Model
	Engine  *swarm.Engine
	Graph   *graph.Graph

	// state
	state      ViewState
	scanning   bool
	quitting   bool
	err        error
	width      int
	height     int
	isTrial    bool
	isMock     bool

	// data
	wasteItems   []*graph.Node
	totalSavings float64
	riskScore    int
	tasksDone    int
	tfRepairReady bool

	// filters
	SortMode   string
	FilterMode string

	// navigation
	cursor       int // main list cursor
	detailsScroll int // TODO: implement scroll if needed

	// animation
	tickCount int
}

type tickMsg time.Time

func NewModel(e *swarm.Engine, g *graph.Graph, isTrial bool, isMock bool) Model {
	s := spinner.New()
	s.Spinner = spinner.Points // "Future" style spinner (dots)
	s.Style = special

	return Model{
		spinner:  s,
		scanning: !isMock,
		isTrial:  isTrial,
		isMock:   isMock,
		Engine:   e,
		Graph:    g,
		state:    ViewStateList,
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
