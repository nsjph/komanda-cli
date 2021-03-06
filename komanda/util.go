package komanda

import (
	"fmt"
	"strings"
	"time"

	"github.com/jroimartin/gocui"
	"github.com/mephux/komanda-cli/komanda/client"
	"github.com/mephux/komanda-cli/komanda/color"
	"github.com/mephux/komanda-cli/komanda/command"
	"github.com/mephux/komanda-cli/komanda/config"
	"github.com/mephux/komanda-cli/komanda/helpers"
	"github.com/mephux/komanda-cli/komanda/logger"
	"github.com/mephux/komanda-cli/komanda/share/history"
	"github.com/mephux/komanda-cli/komanda/share/trie"
	"github.com/mephux/komanda-cli/komanda/ui"
)

var (
	curView         = 0
	inCacheTab      = false
	cacheTabIndex   = 0
	cacheTabSearch  = ""
	cacheTabResults = []string{}

	// InputHistory buffer
	InputHistory = history.New()
)

// TODO: fix \x00 issues
func tabUpdateInput(input *gocui.View) (string, bool) {

	// logger.Logger.Println(spew.Sdump(input.Buffer()))

	search := strings.TrimSpace(input.ViewBuffer())
	searchSplit := strings.Split(search, " ")
	search = searchSplit[len(searchSplit)-1]

	if inCacheTab {
		cacheTabIndex++

		if cacheTabIndex > len(cacheTabResults)-1 {
			cacheTabIndex = 0
		}

		searchSplit[len(searchSplit)-1] = cacheTabResults[cacheTabIndex]

		newInputData := strings.Join(searchSplit, " ")

		input.Clear()

		if !strings.HasPrefix(newInputData, "/") && !strings.HasPrefix(newInputData, "#") {
			newInputData = newInputData + ":"
		}

		fmt.Fprint(input, newInputData+" ")
		input.SetCursor(len(input.Buffer())-1, 0)

		// logger.Logger.Println(spew.Sdump(newInputData + ""))
		// logger.Logger.Printf("WORD %s -- %s -- %s\n", search, cacheTabSearch, cacheTabResults[cacheTabIndex])
		return "", true
	}

	return search, false
}

func tabComplete(g *gocui.Gui, v *gocui.View) error {

	input, err := g.View("input")

	if err != nil {
		return err
	}

	search, cache := tabUpdateInput(input)

	if cache {
		return nil
	}

	t := trie.New()

	// Add Commands
	for i, c := range command.Commands {
		md := c.Metadata()
		d := md.Name()
		// var chars = ""

		t.Add(fmt.Sprintf("/%s", d), i)

		for ai, a := range md.Aliases() {
			t.Add(fmt.Sprintf("/%s", a), ai+i)
		}
	}

	// Add Channels
	for channelIndex, channelName := range Server.Channels {
		if channelName.Name != client.StatusChannel {
			t.Add(channelName.Name, fmt.Sprintf("channel-%d", channelIndex))
		}
	}

	// Add Current Chan Users
	if c, _, hasCurrentChannel :=
		Server.HasChannel(Server.CurrentChannel); hasCurrentChannel {

		for userIndex, user := range c.Users {
			if user.Nick != Server.Nick {
				t.Add(user.Nick, fmt.Sprintf("user-%d", userIndex))
			}
		}
	}

	if len(search) <= 0 {
		return nil
	}

	results := t.PrefixSearch(search)

	if len(results) <= 0 {
		inCacheTab = false
		cacheTabSearch = ""
		cacheTabResults = []string{}
		return nil
	}

	inCacheTab = true
	cacheTabSearch = search
	cacheTabResults = results

	_, cache = tabUpdateInput(input)

	if cache {
		return nil
	}

	// logger.Logger.Printf("RESULTS %s -- %s\n", search, results)

	return nil
}

func simpleEditor(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {
	var tab = false
	var inHistroy = false

	switch {
	case key == gocui.KeyTab:
		tab = true
	case ch != 0 && mod == 0:
		v.EditWrite(ch)
	case key == gocui.KeySpace:
		v.EditWrite(' ')
	case key == gocui.KeyBackspace || key == gocui.KeyBackspace2:
		v.EditDelete(true)
	case key == gocui.KeyDelete:
		v.EditDelete(false)
	case key == gocui.KeyInsert:
		v.Overwrite = !v.Overwrite
	case key == gocui.KeyEnter:

		if line := v.ViewBuffer(); len(line) > 0 {
			GetLine(Server.Gui, v)
		} else {
			if c, err := Server.Gui.View(Server.CurrentChannel); err == nil {
				c.Autoscroll = true
			}
		}

		InputHistory.Current()

		// v.EditNewLine()
		// v.Rewind()

	// case key == gocui.MouseMiddle:
	// nextView(Server.Gui, v)
	// case key == gocui.MouseRight:

	case key == gocui.KeyArrowDown:
		inHistroy = true

		if line := InputHistory.Next(); len(line) > 0 {
			v.Clear()
			v.SetCursor(0, 0)
			v.SetOrigin(0, 0)

			fmt.Fprint(v, line)
			v.SetCursor(len(v.Buffer())-1, 0)
		}
	case key == gocui.KeyArrowUp:
		inHistroy = true

		if line := InputHistory.Prev(); len(line) > 0 {
			v.Clear()
			v.SetCursor(0, 0)
			v.SetOrigin(0, 0)

			fmt.Fprint(v, line)
			v.SetCursor(len(v.Buffer())-1, 0)
		}
	case key == gocui.KeyArrowLeft:
		v.MoveCursor(-1, 0, false)
	case key == gocui.KeyArrowRight:

		cx, _ := v.Cursor()
		line := v.ViewBuffer()

		// logger.Logger.Println(len(line), cx)
		// logger.Logger.Println(spew.Sdump(line))

		// if cx == 0 {
		// v.MoveCursor(-1, 0, false)
		if cx < len(line)-1 {
			v.MoveCursor(1, 0, false)
		}

	case key == gocui.KeyCtrlA:
		v.SetCursor(0, 0)
		v.SetOrigin(0, 0)
	case key == gocui.KeyCtrlK:
		v.Clear()
		v.SetCursor(0, 0)
		v.SetOrigin(0, 0)
	case key == gocui.KeyCtrlE:
		v.SetCursor(len(v.Buffer())-1, 0)
	case key == gocui.KeyCtrlLsqBracket:
		// logger.Logger.Println("word...")
	}

	if !inHistroy {
		// InputHistory.Current()
	}

	if !tab {
		// logger.Logger.Print("CALL\n")

		inCacheTab = false
		cacheTabSearch = ""
		cacheTabResults = []string{}
	}
}

// GetLine and send the content to the current view
func GetLine(g *gocui.Gui, v *gocui.View) error {
	// _, cy := v.Cursor()
	// if line, err = v.Line(0); err != nil {
	// line = ""
	// }

	line := v.Buffer()

	// logger.Logger.Printf("LINE %s\n", line)

	if len(line) <= 0 {
		// return errors.New("input line empty")
		v.Clear()
		v.SetCursor(0, 0)
		return nil
	}

	line = strings.Replace(line, "\x00", "", -1)
	line = strings.Replace(line, "\n", "", -1)

	if len(line) <= 0 {
		// return errors.New("input line empty")
		v.Clear()
		v.SetCursor(0, 0)
		v.SetOrigin(0, 0)
		return nil
	}

	InputHistory.Add(line)

	if strings.HasPrefix(line, "//") || !strings.HasPrefix(line, "/") {
		if len(Server.CurrentChannel) > 0 {

			Server.Exec(Server.CurrentChannel, func(c *client.Channel, g *gocui.Gui, v *gocui.View, s *client.Server) error {
				if Server.Client.Connected() {
					// logger.Logger.Println("SEND:::", spew.Sdump(line))

					go Server.Client.Privmsg(Server.CurrentChannel, line)
				}
				return nil
			})

			mainView, err := g.View(Server.CurrentChannel)

			if err != nil {
				return err
			}

			if mainView.Name() != client.StatusChannel {

				Server.Exec(mainView.Name(), func(c *client.Channel, g *gocui.Gui, v *gocui.View, s *client.Server) error {

					timestamp := time.Now().Format(config.C.Time.MessageFormat)

					// logger.Logger.Println(spew.Sdump(color.String(color.TimestampColor, timestamp)))
					// logger.Logger.Println(c.Name, spew.Sdump(c.Users))

					fmt.Fprintf(mainView, "[%s] -> %s: %s\n",
						color.String(config.C.Color.Timestamp, timestamp),
						color.String(config.C.Color.MyNick, c.FindUser(Server.Client.Me().Nick).String(false)),
						// color.String(
						// color.MyNickColor,
						// c.FindUser(Server.Client.Me().Nick).String(false),
						// ),
						color.String(config.C.Color.MyText, helpers.FormatMessage(line)))

					return nil
				})
			}
		}
		// send text
	} else {
		l := strings.Replace(line[1:], "\x00", "", -1)
		l = strings.Replace(l, "\n", "", -1)
		split := strings.Split(strings.TrimRight(l, " "), " ")

		// logger.Logger.Println("IN COMMAND!!!", line, spew.Sdump(split))

		// mainView, _ := g.View(client.StatusChannel)
		// fmt.Fprintln(mainView, "$ COMMAND = ", split[0], len(split))

		// TODO: what was this?
		if len(split) <= 1 {
			if split[0] == "p" || split[0] == "part" || split[0] == "q" {
				command.Run(split[0], []string{"", Server.CurrentChannel})
				v.Clear()
				v.SetCursor(0, 0)
				return nil
			}
		}

		if err := command.Run(split[0], split); err != nil {
			client.StatusMessage(v, err.Error())
		}
	}

	// idleInputText := fmt.Sprintf("[%s] ", client.StatusChannel)

	// if len(command.CurrentChannel) > 0 {
	// idleInputText = fmt.Sprintf("[%s] ", command.CurrentChannel)
	// }

	// fmt.Fprint(v, idleInputText)
	// v.SetCursor(len(idleInputText), 0)

	v.Clear()
	v.SetCursor(0, 0)
	v.SetOrigin(0, 0)

	inCacheTab = false
	cacheTabSearch = ""
	cacheTabResults = []string{}

	FocusAndResetAll(g, v)

	return nil
}

// ScrollUp view by one
func ScrollUp(g *gocui.Gui, cv *gocui.View) error {
	v, _ := g.View(Server.CurrentChannel)
	ScrollView(v, -1)
	return nil
}

// ScrollDown view by one
func ScrollDown(g *gocui.Gui, cv *gocui.View) error {
	v, _ := g.View(Server.CurrentChannel)
	ScrollView(v, 1)
	return nil
}

// ScrollView by a given offset
func ScrollView(v *gocui.View, dy int) error {
	if v != nil {
		v.Autoscroll = false
		ox, oy := v.Origin()
		if err := v.SetOrigin(ox, oy+dy); err != nil {
			return err
		}
	}

	return nil
}

// FocusStatusView pus the status window ontop
func FocusStatusView(g *gocui.Gui, v *gocui.View) error {

	v.Autoscroll = true

	if _, err := g.SetCurrentView(client.StatusChannel); err != nil {
		return err
	}

	return nil
}

// FocusInputView puts the input view ontop
func FocusInputView(g *gocui.Gui, v *gocui.View) error {

	v.SetCursor(len(v.Buffer()+"")-1, 0)

	if _, err := g.SetCurrentView("input"); err != nil {
		return err
	}

	return nil
}

// FocusAndResetAll will put the status and input views ontop and
// reset the input content
func FocusAndResetAll(g *gocui.Gui, v *gocui.View) error {
	status, _ := g.View(client.StatusChannel)
	input, _ := g.View("input")

	FocusStatusView(g, status)
	FocusInputView(g, input)
	return nil
}

func nextView(g *gocui.Gui, v *gocui.View) error {
	curView = getCurrentChannelIndex()

	next := curView + 1

	if next > len(Server.Channels)-1 {
		next = 0
	}

	logger.Logger.Printf("NEXT INDEX %d\n", next)

	newView, err := g.View(Server.Channels[next].Name)

	if err != nil {
		return err
	}

	newView.Autoscroll = true
	g.SetViewOnTop(newView.Name())
	g.SetViewOnTop("header")

	if _, err := g.SetCurrentView(Server.Channels[next].Name); err != nil {
		return err
	}

	// logger.Logger.Printf("Set Current View %d\n", Server.Channels[next].Name)
	Server.CurrentChannel = Server.Channels[next].Name
	Server.Channels[next].Unread = false
	Server.Channels[next].Highlight = false

	ui.UpdateMenuView(g)
	FocusInputView(g, v)

	curView = next
	return nil
}

// TODO: find a better way to do this.
func nextViewActive(g *gocui.Gui, v *gocui.View) error {

	inputView, err := g.View("input")

	if err != nil {
		return err
	}

	// return because the input view has data
	if len(inputView.Buffer()) > 0 {
		return nil
	}

	curView = getCurrentChannelIndex()

	if curView >= len(Server.Channels)-1 {
		curView = 0
	}

	change := func(index int, channel *client.Channel) (bool, error) {
		if index >= curView {
			if channel.Unread || channel.Highlight {

				view, err := channel.View()

				if err != nil {
					return false, err
				}

				view.Autoscroll = true
				g.SetViewOnTop(view.Name())
				g.SetViewOnTop("header")

				if _, err := g.SetCurrentView(channel.Name); err != nil {
					return false, err
				}

				// logger.Logger.Printf("Set Current View %d\n", Server.Channels[next].Name)
				Server.CurrentChannel = channel.Name
				channel.Unread = false
				channel.Highlight = false

				ui.UpdateMenuView(g)
				FocusInputView(g, v)

				return true, nil
			}
		}

		return false, nil
	}

	for index, channel := range Server.Channels {
		has, err := change(index, channel)

		if err != nil {
			return err
		}

		if has {
			return nil
		}

		curView = index
	}

	if curView == len(Server.Channels) {
		curView = 0

		for index, channel := range Server.Channels {
			has, err := change(index, channel)

			if err != nil {
				return err
			}

			if has {
				return nil
			}
		}
	}

	return nil
}

func prevView(g *gocui.Gui, v *gocui.View) error {
	curView = getCurrentChannelIndex()

	next := curView - 1

	if next < 0 {
		next = len(Server.Channels) - 1
	}

	logger.Logger.Printf("PREV INDEX %d\n", next)

	newView, err := g.View(Server.Channels[next].Name)

	if err != nil {
		return err
	}

	newView.Autoscroll = true
	g.SetViewOnTop(newView.Name())
	g.SetViewOnTop("header")

	if _, err := g.SetCurrentView(Server.Channels[next].Name); err != nil {
		return err
	}

	// logger.Logger.Printf("Set Current View %d\n", Server.Channels[next].Name)
	Server.CurrentChannel = Server.Channels[next].Name
	Server.Channels[next].Unread = false
	Server.Channels[next].Highlight = false

	ui.UpdateMenuView(g)
	FocusInputView(g, v)

	curView = next
	return nil
}

func setView(g *gocui.Gui, v *gocui.View, index int) error {

	c := Server.Channels[index]

	if c != nil {

		newView, err := g.View(c.Name)

		if err != nil {
			return err
		}

		newView.Autoscroll = true
		g.SetViewOnTop(newView.Name())
		g.SetViewOnTop("header")

		if _, err := g.SetCurrentView(c.Name); err != nil {
			return err
		}

		Server.CurrentChannel = c.Name
		c.Unread = false
		c.Highlight = false

		ui.UpdateMenuView(g)
		FocusInputView(g, v)
	}

	return nil
}

func getCurrentChannelIndex() int {
	for i, s := range Server.Channels {
		if s.Name == Server.CurrentChannel {
			return i
		}
	}

	return 0
}

func moveView(g *gocui.Gui, v *gocui.View, dx, dy int) error {

	name := v.Name()

	// x0, y0, x1, y1, err := g.ViewPosition(name)
	// if err != nil {
	// return err
	// }

	// logger.Logger.Printf("RESIZE %d %d %d %d\n", x0+dx, y0+dy, x1+dx, y1+dy)

	if _, err := g.SetView(name, 0, 0, 0, 0); err != nil {
		return err
	}

	return nil
}
