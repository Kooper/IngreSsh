package server

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gliderlabs/ssh"
	log "github.com/sirupsen/logrus"
	"kuberstein.io/ingressh/internal/types"
)

const (
	defaultListHeight = 20
	defaultListWidth  = 60
)

var (
	titleStyle        = lipgloss.NewStyle().MarginLeft(2)
	itemStyle         = lipgloss.NewStyle().PaddingLeft(4)
	selectedItemStyle = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("170"))
	paginationStyle   = list.DefaultStyles().PaginationStyle.PaddingLeft(4)
	helpStyle         = list.DefaultStyles().HelpStyle.PaddingLeft(4).PaddingBottom(1)
	quitTextStyle     = lipgloss.NewStyle().Margin(1, 0, 2, 4)
)

type item string

func (i item) FilterValue() string { return "" }

type itemDelegate struct{}

func (d itemDelegate) Height() int                             { return 1 }
func (d itemDelegate) Spacing() int                            { return 0 }
func (d itemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(item)
	if !ok {
		return
	}

	str := fmt.Sprintf("%d. %s", index+1, i)

	fn := itemStyle.Render
	if index == m.Index() {
		fn = func(s ...string) string {
			return selectedItemStyle.Render("> " + strings.Join(s, " "))
		}
	}

	fmt.Fprint(w, fn(str))
}

type selectionState int

const (
	selectNamespace selectionState = iota
	selectPod
	selectContainer
)

type model struct {
	state      selectionState
	stateNoWay error

	width int

	listNamespaces list.Model
	listPods       list.Model
	listContainers list.Model

	choiceNamespace string
	choicePod       string
	choiceContainer string

	quitting          bool
	quittingWithError error

	targetAuth authz
	hint       types.SshTarget

	podConfigs        []podSshConfig
	selectedPodConfig podSshConfig
}

func (m model) Init() tea.Cmd {
	m.width = defaultListWidth
	m.state = selectNamespace
	return nil
}

func (m *model) activeList() *list.Model {
	var a *list.Model
	switch m.state {
	case selectNamespace:
		a = &m.listNamespaces
	case selectPod:
		a = &m.listPods
	case selectContainer:
		a = &m.listContainers
	}
	return a
}

func (m model) setupList(items []list.Item, title string) list.Model {
	l := list.New(items, itemDelegate{}, m.width, defaultListHeight)
	l.Title = title
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = titleStyle
	l.Styles.PaginationStyle = paginationStyle
	l.Styles.HelpStyle = helpStyle
	return l
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {

	activeList := m.activeList()

	// Common key handlers
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		activeList.SetWidth(msg.Width)
		return m, nil

	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}
	}

	// Key handlers that depends on the state

	// "Nothing here" screen
	if m.stateNoWay != nil {
		switch msg.(type) {
		case tea.KeyMsg:
			// "Nothing here" screen is shown. Stays on the current list as
			// stateNoWay is raised when the next list for the selected object
			// can't be created. Drop the flag for the "nothing here" screen.
			m.stateNoWay = nil
		}
		return m, nil
	}

	// Regular wizard lists state
	switch msg := msg.(type) {

	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {

		case "enter":
			i, ok := activeList.SelectedItem().(item)
			if !ok {
				return m, tea.Quit
			}

			switch m.state {
			case selectNamespace:
				m.stateNoWay = m.stepSelectPod()
			case selectPod:
				m.stateNoWay = m.stepSelectContainer()
			case selectContainer:
				m.choiceContainer = string(i)
				return m, tea.Quit
			}

		case "esc":
			// Escape brings us to the previous state of the selection wizard
			switch m.state {
			case selectNamespace:
				m.quitting = true
				return m, tea.Quit
			case selectPod:
				m.state = selectNamespace
				m.choiceNamespace = ""
			case selectContainer:
				m.state = selectPod
				m.choicePod = ""
			}
		}

		return m, nil
	}

	var cmd tea.Cmd
	*activeList, cmd = activeList.Update(msg)

	return m, cmd
}

func (m model) View() string {

	if m.stateNoWay != nil {
		return quitTextStyle.Render(fmt.Sprintf("No authorized objects: %s. Press any key to select a different option\n", m.stateNoWay))
	}

	if m.choiceContainer != "" {
		return quitTextStyle.Render(fmt.Sprintf("Proceed with %s/%s/%s...", m.choiceNamespace, m.choicePod, m.choiceContainer))
	}
	if m.quittingWithError != nil {
		return quitTextStyle.Render(fmt.Sprintf("Error setting up SSH session: %v", m.quittingWithError))
	}
	if m.quitting {
		return quitTextStyle.Render("SSH session setup has been cancelled")
	}
	activeList := m.activeList()
	return "\n" + activeList.View()
}

func (m *model) stepSelectPod() error {

	targetNamespace := string(m.listNamespaces.SelectedItem().(item))
	podConfigs, err := m.targetAuth.GetPods(targetNamespace, m.hint.Pod)
	if err != nil {
		return err
	}
	if len(podConfigs) == 0 {
		return fmt.Errorf("No authorized pods in ns %s", targetNamespace)
	}

	m.choiceNamespace = targetNamespace
	m.podConfigs = podConfigs

	items := []list.Item{}
	for _, p := range podConfigs {
		items = append(items, item(p.pod.Name))
	}

	m.listPods = m.setupList(items, fmt.Sprintf("Select a pod in the ns '%s'", targetNamespace))
	m.state = selectPod

	return nil
}

func (m *model) stepSelectContainer() error {

	podConfigIdx := m.listPods.Index()

	selectedPodConfig := m.podConfigs[podConfigIdx]
	pod := selectedPodConfig.pod
	config := selectedPodConfig.config
	containers, err := m.targetAuth.GetContainers(pod, config.Containers, m.hint.Container)
	if err != nil {
		return err
	}

	if len(containers) == 0 {
		return fmt.Errorf("No authorized containers in pod %s", pod.Name)
	}

	m.choicePod = string(m.listPods.SelectedItem().(item))
	m.selectedPodConfig = m.podConfigs[podConfigIdx]

	items := []list.Item{}
	for _, c := range containers {
		items = append(items, item(c))
	}

	m.listContainers = m.setupList(items, fmt.Sprintf("Select a container in %s/%s", m.choiceNamespace, m.choicePod))
	m.state = selectContainer

	return nil
}

func (m *model) stepSelectNamespace() error {

	namespaces, err := m.targetAuth.GetNamespaces(m.hint.Namespace)
	if err != nil {
		return err
	}
	if len(namespaces) == 0 {
		return errors.New("No authorized namespaces")
	}

	items := []list.Item{}
	for _, ns := range namespaces {
		items = append(items, item(ns))
	}

	m.listNamespaces = m.setupList(items, "Select the namespace")
	m.state = selectNamespace
	return nil
}

func (m model) result() (types.SshTarget, podSshConfig) {
	r := types.SshTarget{
		Namespace: m.choiceNamespace,
		Pod:       m.choicePod,
		Container: m.choiceContainer,
	}
	return r, m.selectedPodConfig
}

// Returns attach target and pod+configuration as a result of the
// user's interactive selection.
// If the user have specified hint information, the appropriate filtering
// is applied to the selection lists.
func interactive(sess ssh.Session, targetAuth authz, hint types.SshTarget) (
	types.SshTarget, podSshConfig, error,
) {

	m := model{
		targetAuth: targetAuth,
		hint:       hint,
	}

	err := m.stepSelectNamespace()
	if err != nil {
		return types.SshTarget{}, podSshConfig{}, err
	}

	p := tea.NewProgram(m, tea.WithOutput(sess), tea.WithInput(sess))
	result, err := p.Run()
	if err != nil {
		log.Errorf("Error on interactive access target select: %v", err)
		return types.SshTarget{}, podSshConfig{}, err
	}

	fmt.Fprint(sess, "\n")
	sshTarget, podConfig := result.(model).result()
	return sshTarget, podConfig, nil
}
