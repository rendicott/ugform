package ugform

import (
	"context"
	"errors"
	"github.com/gdamore/tcell/v2"
	"github.com/inconshreveable/log15"
	"sort"
	"time"
)

// Loggo is the global logger. Set this to a log15
// logger from your main to incorporate into main
// logfile. Otherwise log messages are discarded
var Loggo log15.Logger

func log(lev, msg string, ltx ...interface{}) {
	if Loggo != nil {
		switch lev {
		case "Info":
			Loggo.Info(msg, ltx...)
		case "Error":
			Loggo.Error(msg, ltx...)
		case "Debug":
			Loggo.Debug(msg, ltx...)
		}
	}
}

// StyleHelper takes a foreground and background color string and
// converts it to a tcell Style struct
func StyleHelper(fgcolor, bgcolor string) (style tcell.Style) {
	var st tcell.Style
	if fgcolor != "" {
		colorFg := tcell.GetColor(fgcolor)
		st = st.Foreground(colorFg)
	} else {
		st.Foreground(tcell.ColorReset)
	}
	if bgcolor != "" {
		colorBg := tcell.GetColor(bgcolor)
		st = st.Background(colorBg)
	} else {
		st.Background(tcell.ColorReset)
	}
	return st
}

// StyleCursor is a helper function that takes a single
// bgcolor input and returns
// a tcell Style. Usefull for remembering that setting
// foreground on a cursor is pointless because no text
// is ever displayed within a cursor.
func StyleCursor(bgcolor string) (style tcell.Style) {
	return StyleHelper("black", bgcolor)
}

// StyleFill is a helper function that takes a single
// bgcolor input and returns
// a tcell Style. Usefull for remembering that setting
// foreground on a textbox fill is pointless because no text
// is ever displayed using that style
func StyleFill(bgcolor string) (style tcell.Style) {
	return StyleHelper("black", bgcolor)
}

// csr takes a string and converts it to a rune slice
// then grabs the rune at index 0 in the slice so that it can return
// an int32 to satisfy the Uggly protobuf struct for border and fill chars
// and such. If the input string is less than zero length then it will just
// rune out a space char and return that int32.
func csr(s string) int32 {
	if len(s) == 0 {
		s = " "
	}
	runes := []rune(s)
	return runes[0]
}

type textBox struct {
	tabOrder          int
	name, description string
	def               string // default to populate contents
	con               []rune
	px, py, pw, ph    int          // textBox position and dimensions
	cx, cy            int          // cursor position
	cs, ts, fs, ds    tcell.Style  // cursor, text, fill, and description style
	s                 tcell.Screen // need direct access to screen
	showDescription   bool
	mask              bool // if password box then mask while typing
}

// remove handles the removal of a rune from the content
// slice for the backspace scenario
func (t *textBox) remove(pos int) {
	t.con = append(t.con[:pos], t.con[pos+1:]...)
}

// setBox handles drawing of the textBox's container on the screen
func (t *textBox) setBox() {
	for i := t.px; i <= t.px+t.pw; i++ {
		t.s.SetContent(i, t.py, csr(""), nil, t.fs)
	}
}

// hideCursor hides the cursor in its current position
func (t *textBox) hideCursor() {
	t.s.SetContent(t.cx, t.cy, csr(""), nil, t.fs)
	t.s.Show()
}

// showCursor shows the cursor in its current position
func (t *textBox) showCursor() {
	t.s.SetContent(t.cx, t.cy, csr(" "), nil, t.cs)
	t.s.Show()
}

// setCursor changes the position of the cursor
func (t *textBox) setCursor(x, y int) {
	t.s.SetContent(x, y, csr(" "), nil, t.cs)
}

// drawDescription draws the textBox's description property
// to the left of the textBox itself. One must accomodate manually
// for the length of the description as this will happily draw
// all the way up to the edge of the screen
func (t *textBox) drawDescription() {
	if t.showDescription {
		sin := make(map[int]rune)
		slen := 0
		for j, r := range t.description {
			slen++
			sin[j] = r
		}
		pos := 0
		offset := slen + 2
		start := t.px - offset
		for i := start; i < start+slen; i++ {
			if i >= 0 {
				t.s.SetContent(i, t.py, sin[pos], nil, t.ds)
			}
			pos += 1
		}
	}
}

// start draws the textbox, description, and hides the cursor
func (t *textBox) start() {
	t.setBox()
	t.drawDescription()
	t.drawText()
	if t.def != "" && len(t.con) == 0 {
		for _, r := range t.def {
			t.add(r)
		}
	}
	t.hideCursor()
	t.s.Show()
}

// drawText draws the text within the textBox and respects
// the boundaries of the box in which it is contained. It takes
// special care to handle the sliding window of text when the text
// length exceeds the length of the containing box.
func (t *textBox) drawText() {
	var pos int
	for i := t.px; i < t.px+t.pw; i++ {
		if len(t.con) > t.pw {
			pos = (len(t.con) - t.pw) + (i - t.px)
		} else if len(t.con) <= t.pw {
			pos = i - t.px
		} else if i-t.px > len(t.con) {
			break
		}
		if len(t.con) > pos {
			var char rune
			if t.mask {
				char = csr("*")
			} else {
				char = t.con[pos]
			}
			t.s.SetContent(i, t.py, char, nil, t.ts)
		}
	}
	// make sure to set cursor now that it's cleard out
	t.setCursor(t.cx, t.cy)
	t.s.Show()
}

// add handles adding contents to the textBox's contents
// as new runes are typed and also handles cursor positioning
func (t *textBox) add(r rune) {
	t.con = append(t.con, r)
	if len(t.con) <= t.pw {
		t.cx += 1
	}
	t.setCursor(t.cx, t.cy)
	t.drawText()
}

// back handles removal of runes from the textBox's contents
// as well as handling cursor position and stopping the cursor
// if the left edge of the box is hit
func (t *textBox) back() {
	if len(t.con) > 0 {
		t.remove(len(t.con) - 1)
		if len(t.con) < t.pw {
			t.cx -= 1
			t.s.SetContent(t.cx+1, t.cy, csr(""), nil, t.ts)
		}
		t.setCursor(t.cx, t.cy)
	}
	t.drawText()
}

// AddTextBox is a constructor for adding a new textBox to the form
// paying special attention to setting up tabOrder, instantiating
// contents, and setting focus. The last box to be added that has the
// hasFocus property set will retain the focus.
func (f *Form) AddTextBox(in *AddTextBoxInput) (err error) {
	t := textBox{}
	t.name = in.Name
	t.description = in.Description
	t.tabOrder = in.TabOrder
	f.tabOrder[t.tabOrder] = t.name
	t.s = f.s
	t.con = make([]rune, 0)
	t.def = in.DefaultValue
	t.px = in.PositionX
	t.py = in.PositionY
	t.pw = in.Width
	t.ph = in.Height
	t.cx = t.px
	t.cy = t.py
	t.cs = in.StyleCursor
	t.fs = in.StyleFill
	t.ts = in.StyleText
	t.ds = in.StyleDescription
	t.mask = in.Password
	t.showDescription = in.ShowDescription
	f.textBoxes[t.name] = &t
	if in.HasFocus {
		f.focus = &t
	}
	return err
}

// AddTextBoxInput provides all the input parameters for the
// AddTextBox constructor
type AddTextBoxInput struct {
	// Name of the textbox which will be included
	// when collecting results
	Name string

	// Description of the textbox which can be shown to the user
	// if desired which will be displayed to the left of the
	// actual textBox. This will be drawn relative to the PositionX
	// of the textBox itself so plan ahead in your design.
	Description string

	// The DefaultValue will be pre-populated if desired
	DefaultValue string

	// TabOrder of the textbox for this form. Must be unique
	// within a form or unstable tab behavior could result
	TabOrder int

	// PositionX is the x-axis position of the textbox
	PositionX int

	// PositionY is the y-axis position of the textbox
	PositionY int

	// Width is the width of the textbox. Values typed into
	// the textbox have virtually unlimited length but only
	// width number of chars will be displayed to the
	// user so provide enough room for comfortable usage
	Width int

	// Height is the height of the textbox. Currently only one
	// line is supported but you could display a taller box if
	// you wanted to I guess
	Height int

	// tcell Style to use for cursor color. Setting the foreground
	// of the Cursor is pointless as it never contains text.
	StyleCursor tcell.Style

	// tcell Style to use for textbox fill color. Setting the
	// foreground of Fill is pointless as it never contains text
	StyleFill tcell.Style

	// tcell Style to use for text color within the textbox.
	// You probably want StyleText bgcolor to match
	// StyleFill's bgcolor.
	StyleText tcell.Style

	// tcell Style for the textbox's description.
	// This uses both foreground and background.
	StyleDescription tcell.Style

	// Whether or not to show the description to the user.
	// This gets written out as PositionX minus length of
	// description so design accordingly.
	ShowDescription bool

	// Whether or not this textBox has focus when the form's
	// polling method is activated. The last textbox to be
	// added to the form that has this set to true will be the
	// first one with focus. If no focus is specified then one
	// will be selected at random.
	HasFocus bool

	// Indicates whether or not this textbox is a password
	// field which will mask it's contents while typing.
	Password bool
}

// Form contains properties and methods for interacting with its
// associated text boxes.
type Form struct {
	// Optional: name for this form. Useful for managing
	// lists of forms for example.
	Name         string
	SubmitAction interface{}
	textBoxes    map[string]*textBox
	tabOrder     map[int]string
	focus        *textBox // the textbox that has focus
	interrupt    chan struct{}
	s            tcell.Screen
}

// Start activates all of the form's components and renders
// to the provided screen. If no focus is specified then focus
// is randomly selected.
func (f *Form) Start() (err error) {
	if len(f.textBoxes) == 0 {
		err = errors.New("no textboxes in form cannot start")
		return err
	}
	if f.focus == nil {
		// since maps are unordered we have to build an ordered index
		var keys []int
		for k, _ := range f.tabOrder {
			keys = append(keys, k)
		}
		sort.Ints(keys)
		if len(keys) > 0 {
			log("Debug", "no focus specified so picking lowest taborder")
			tbNameAtIndex := f.tabOrder[keys[0]]
			f.focus = f.textBoxes[tbNameAtIndex]
		} else {
			log("Debug", "can't order taborder so picking random focus")
			for _, tb := range f.textBoxes {
				f.focus = tb
			}
		}
	}
	for _, tb := range f.textBoxes {
		tb.start()
	}
	return err
}

func (f *Form) tab(direction string) {
	if direction != "forward" && direction != "backward" {
		log("Debug", "detected non-supported direction", "direction", direction)
		return
	}
	// since maps are unordered we have to build an ordered index
	var keys []int
	f.focus.hideCursor()
	for k, _ := range f.tabOrder {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	var pos int
	for i, k := range keys {
		if f.focus.tabOrder == k && direction == "forward" {
			pos = i + 1
		}
		if f.focus.tabOrder == k && direction == "backward" {
			pos = i - 1
		}
	}
	if pos >= len(keys) && direction == "forward" {
		pos = 0
	}
	if pos < 0 && direction == "backward" {
		pos = len(keys) - 1
	}
	next := f.tabOrder[keys[pos]]
	log("Debug", "tab", "next", next)
	f.focus = f.textBoxes[next]
	f.focus.showCursor()
}

// Collect returns a map of the name and contents of all of the form's
// textboxes.
func (f *Form) Collect() (results map[string]string) {
	results = make(map[string]string)
	for _, v := range f.textBoxes {
		results[v.name] = string(v.con)
	}
	return results
}

// NewForm instantiates a new form and returns a pointer
// to which textBoxes can be added and the other various
// Form methods can be used. Once a Form is created and
// populated with textboxes using the AddTextBox method
// then the Start() method should be called followed by
// the Poll() method.
func NewForm(s tcell.Screen) (f *Form) {
	nf := Form{}
	nf.s = s
	nf.textBoxes = make(map[string]*textBox)
	nf.tabOrder = make(map[int]string)
	return &nf
}

// ShiftXY shifts all coordinates within the form by
// x and y pixels. This is useful when you're drawing
// forms relative to other elements on the screen. This
// method will not clear the screen so if that's desired
// you should do it manually.
func (f *Form) ShiftXY(x, y int) {
	for _, tb := range f.textBoxes {
		tb.px += x
		tb.cx += x
		tb.py += y
		tb.cy += y
	}
}

// Clears the screen then calls the ShiftXY function then
// redraws
func (f *Form) ClearShiftXY(x, y int) {
	f.s.Clear()
	f.ShiftXY(x, y)
	f.Start()
}

// AddSampleTextBoxes takes an existing form and then adds some
// basic sample textBoxes
func AddSampleTextBoxes(nf *Form) (err error) {
	err = nf.AddTextBox(&AddTextBoxInput{
		TabOrder:         0,
		Name:             "test1",
		DefaultValue:     "Joe",
		Description:      "What is the name of your favorite childhood friend?",
		PositionX:        80,
		PositionY:        5,
		Height:           1,
		Width:            10,
		StyleCursor:      StyleHelper("black", "white").Blink(true),
		StyleFill:        StyleHelper("black", "grey"),
		StyleText:        StyleHelper("black", "grey"),
		StyleDescription: StyleHelper("white", "black"),
		ShowDescription:  true,
	})
	if err != nil {
		return err
	}
	err = nf.AddTextBox(&AddTextBoxInput{
		TabOrder:         2,
		Name:             "test2",
		Description:      "Where did you grow up?",
		PositionX:        80,
		PositionY:        7,
		Height:           1,
		Width:            20,
		StyleCursor:      StyleHelper("black", "white").Blink(true),
		StyleFill:        StyleHelper("black", "grey"),
		StyleText:        StyleHelper("black", "grey"),
		StyleDescription: StyleHelper("white", "black"),
		ShowDescription:  true,
		//HasFocus:         true,
	})
	if err != nil {
		return err
	}
	err = nf.AddTextBox(&AddTextBoxInput{
		TabOrder:         4,
		Name:             "test3",
		DefaultValue:     "super long value",
		Description:      "Age",
		PositionX:        80,
		PositionY:        9,
		Height:           1,
		Width:            5,
		StyleCursor:      StyleHelper("black", "white").Blink(true),
		StyleFill:        StyleHelper("black", "grey"),
		StyleText:        StyleHelper("black", "grey"),
		StyleDescription: StyleHelper("white", "black"),
		ShowDescription:  true,
	})
	if err != nil {
		return err
	}
	err = nf.AddTextBox(&AddTextBoxInput{
		TabOrder:         7,
		Name:             "test4",
		Description:      "Weight",
		PositionX:        80,
		PositionY:        11,
		Height:           1,
		Width:            5,
		StyleCursor:      StyleHelper("black", "white").Blink(true),
		StyleFill:        StyleHelper("black", "grey"),
		StyleText:        StyleHelper("black", "grey"),
		StyleDescription: StyleHelper("white", "black"),
		ShowDescription:  true,
	})
	return err
}

func (f *Form) ctxWatcher(ctx context.Context, die chan int) {
	for {
		select {
		case <-ctx.Done():
			log("Debug", "1. caught done signal from ctx")
			f.s.PostEvent(fakeEvent{})
			// send a fake event since main poll blocking on PollEvent
			return
		case <-die:
			log("Debug", "2. ctxWatcher caught die signal")
			return
		}
	}
}

type fakeEvent struct{}

func (f fakeEvent) When() time.Time {
	return time.Now()
}

/*Poll handles the keyboard events related to the form. Ideally you would
cede control over to the Form's polling loop and trust it to return
control back to another polling loop. It takes an interrupt channel
parameter which you should pass it and then have your main polling
loop block waiting for the form's interrupt channel to close.*/
func (f *Form) Poll(ctx context.Context, interrupt chan struct{}, submit chan string) {
	die := make(chan int)
	go f.ctxWatcher(ctx, die)
	log("Info", "starting form poll", "formName", f.Name)
	f.focus.showCursor()
	for {
		log("Debug", "blocking on PollEvent()")
		ev := f.s.PollEvent()
		log("Debug", "caught event")
		switch ev := ev.(type) {
		case *tcell.EventKey:
			switch ev.Key() {
			case tcell.KeyRune:
				log("Debug", "detected typing")
				f.focus.add(ev.Rune())
			case tcell.KeyTab:
				f.tab("forward")
			case tcell.KeyBacktab:
				f.tab("backward")
			case tcell.KeyBackspace, tcell.KeyBackspace2:
				log("Debug", "detected backspace")
				f.focus.back()
			case tcell.KeyEnter:
				// submit form name to given channel
				log("Info", "sending to submit channel")
				// indicating desire to Collect()
				submit <- f.Name
				// run normal exit procedure
				f.focus.hideCursor()
				close(interrupt)
				die <- 0
				close(die)
				return
			case tcell.KeyEscape:
				// means we're exiting form focus
				f.focus.hideCursor()
				close(interrupt)
				die <- 0
				close(die)
				return
			default:
				log("Debug", "detected stroke", "keyStroke", ev.Name())
			}
		case fakeEvent:
			f.focus.hideCursor()
			close(interrupt)
			return
		}
	}
}
