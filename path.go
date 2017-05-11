package svg

import (
	"fmt"
	"strconv"
)

// Path is
type Path struct {
	ID          string `xml:"id,attr"`
	D           string `xml:"d,attr"`
	Style       string `xml:"style,attr"`
	properties  map[string]string
	strokeWidth float64
	command     chan *Command
	group       *Group
}

//
const (
	MOVETO = iota
	LINETO
	CURVETO
)

// Command is
type Command struct {
	Name   int
	Points []Tuple
}

func (c *Command) addPoint(p Tuple) {
	c.Points = append(c.Points, p)
}

type pathDescriptionParser struct {
	p              *Path
	lex            Lexer
	x, y           float64
	currentcommand int
	tokbuf         [4]Item
	peekcount      int
	lasttuple      Tuple
	transform      Transform
	svg            *Svg
	x1, y1         float64
	x2, y2         float64
	initX, initY   float64
}

func newPathDParse() *pathDescriptionParser {
	pdp := &pathDescriptionParser{}
	pdp.transform = Identity()
	return pdp
}

// Parse interprets path description, transform and style atttributes to create a channel of segments.
func (p *Path) Parse() chan *Command {
	p.parseStyle()
	pdp := newPathDParse()
	pdp.p = p
	pdp.svg = p.group.Owner
	pdp.transform.MultiplyWith(*p.group.Transform)
	p.command = make(chan *Command)
	l, _ := Lex(fmt.Sprint(p.ID), p.D)
	pdp.lex = *l
	go func() {
		defer close(p.command)
		for {
			i := pdp.lex.NextItem()
			switch {
			case i.Type == ItemError:
				return
			case i.Type == ItemEOS:
				return
			case i.Type == ItemLetter:
				parseCommand(pdp, l, i)
			default:
			}
		}
	}()
	return p.command
}

func parseCommand(pdp *pathDescriptionParser, l *Lexer, i Item) error {
	var err error
	switch i.Value {
	case "M":
		err = parseMoveTo(pdp, true)
	case "m":
		err = parseMoveTo(pdp, false)
	case "c":
		err = parseCurveTo(pdp, false)
	case "C":
		err = parseCurveTo(pdp, true)
	case "S":
		err = parseSCurveTo(pdp, true)
	case "s":
		err = parseSCurveTo(pdp, false)
	case "Q":
		err = parseQCurveTo(pdp, true)
	case "q":
		err = parseQCurveTo(pdp, false)
	case "T":
		err = parseTCurveTo(pdp, true)
	case "t":
		err = parseTCurveTo(pdp, false)
	case "L":
		err = parseLineTo(pdp, true)
	case "l":
		err = parseLineTo(pdp, false)
	case "H":
		err = parseHLineTo(pdp, true)
	case "h":
		err = parseHLineTo(pdp, false)
	case "V":
		err = parseVLineTo(pdp, true)
	case "v":
		err = parseVLineTo(pdp, false)
	case "Z":
		err = parseClose(pdp)
	case "z":
		err = parseClose(pdp)
	default:
		fmt.Println("didn't parse", i.Value)
	}
	if err != nil {
		fmt.Println("parse command error", err)
	}
	if i.Value != "c" && i.Value != "C" && i.Value != "s" && i.Value != "S" {
		pdp.x2 = pdp.x
		pdp.y2 = pdp.y
	}
	if i.Value != "Q" && i.Value != "q" && i.Value != "T" && i.Value != "t" {
		pdp.x1 = pdp.x
		pdp.y1 = pdp.y
	}
	return err

}

func parseMoveTo(pdp *pathDescriptionParser, abs bool) error {
	tuples, err := parseTuples(pdp)
	if err != nil {
		return err
	}
	t := tuples[0]
	if !abs {
		t[0] += pdp.x
		t[1] += pdp.y
	}
	pdp.x = t[0]
	pdp.y = t[1]
	pdp.initX, pdp.initY = t[0], t[1]
	cmd := &Command{
		Name:   MOVETO,
		Points: []Tuple{pdp.transform.Apply(t)},
	}
	pdp.p.command <- cmd
	if len(tuples) > 1 {
		for i := 1; i < len(tuples); i++ {
			t := tuples[i]
			if !abs {
				t[0] += pdp.x
				t[1] += pdp.y
			}
			pdp.x = t[0]
			pdp.y = t[1]
			cmd := &Command{
				Name:   LINETO,
				Points: []Tuple{pdp.transform.Apply(t)},
			}
			pdp.p.command <- cmd
		}
	}
	return nil
}

func parseLineTo(pdp *pathDescriptionParser, abs bool) error {
	tuples, err := parseTuples(pdp)
	if err != nil {
		return err
	}
	for _, t := range tuples {
		if !abs {
			t[0] += pdp.x
			t[1] += pdp.y
		}
		pdp.x = t[0]
		pdp.y = t[1]
		cmd := &Command{
			Name:   LINETO,
			Points: []Tuple{pdp.transform.Apply(t)},
		}
		pdp.p.command <- cmd
	}
	return nil
}

func parseNumbers(pdp *pathDescriptionParser) ([]float64, error) {
	var numbers []float64
	for pdp.lex.PeekItem().Type == ItemNumber {
		t, err := parseNumber(pdp.lex.NextItem())
		if err != nil {
			return nil, err
		}
		numbers = append(numbers, t)
	}
	return numbers, nil
}

func parseTuples(pdp *pathDescriptionParser) ([]Tuple, error) {
	var tuples []Tuple
	pdp.lex.ConsumeWhiteSpace()
	for pdp.lex.PeekItem().Type == ItemNumber {
		t, err := parseTuple(&pdp.lex)
		if err != nil {
			return nil, err
		}
		tuples = append(tuples, t)
		pdp.lex.ConsumeWhiteSpace()
	}
	return tuples, nil
}

func parseHLineTo(pdp *pathDescriptionParser, abs bool) error {
	numbers, err := parseNumbers(pdp)
	if err != nil {
		return err
	}
	for _, n := range numbers {
		t := Tuple{}
		t[1] = pdp.y
		if !abs {
			n += pdp.x
		}
		t[0] = n
		pdp.x = n
		cmd := &Command{
			Name:   LINETO,
			Points: []Tuple{pdp.transform.Apply(t)},
		}
		pdp.p.command <- cmd
	}
	return nil
}

func parseVLineTo(pdp *pathDescriptionParser, abs bool) error {
	numbers, err := parseNumbers(pdp)
	if err != nil {
		return err
	}
	for _, n := range numbers {
		t := Tuple{}
		t[0] = pdp.x
		if !abs {
			n += pdp.y
		}
		t[1] = n
		pdp.y = n
		cmd := &Command{
			Name:   LINETO,
			Points: []Tuple{pdp.transform.Apply(t)},
		}
		pdp.p.command <- cmd
	}
	return nil
}

func parseClose(pdp *pathDescriptionParser) error {
	var t Tuple
	t[0], t[1] = pdp.initX, pdp.initY
	pdp.x, pdp.y = pdp.initX, pdp.initY
	cmd := &Command{
		Name:   LINETO,
		Points: []Tuple{pdp.transform.Apply(t)},
	}
	pdp.p.command <- cmd
	return nil
}

func reflection(x, y, x2, y2 float64) Tuple {
	t := Tuple{}
	t[0] = x + x - x2
	t[1] = y + y - y2
	return t
}

func parseQCurveTo(pdp *pathDescriptionParser, abs bool) error {
	tuples, err := parseTuples(pdp)
	if err != nil {
		return err
	}
	for j := 0; j < len(tuples)/2; j++ {
		if !abs {
			for i := 2 * j; i < 2+2*j; i++ {
				tuples[i][0] += pdp.x
				tuples[i][1] += pdp.y
			}
		}
		x1 := pdp.x + float64(2)/float64(3)*(tuples[0+2*j][0]-pdp.x)
		y1 := pdp.y + float64(2)/float64(3)*(tuples[0+2*j][1]-pdp.y)
		x2 := tuples[1+2*j][0] + float64(2)/float64(3)*(tuples[0+2*j][0]-tuples[1+2*j][0])
		y2 := tuples[1+2*j][1] + float64(2)/float64(3)*(tuples[0+2*j][1]-tuples[1+2*j][1])
		pdp.x = tuples[1+2*j][0]
		pdp.y = tuples[1+2*j][1]
		pdp.x1 = tuples[0+2*j][0]
		pdp.y1 = tuples[0+2*j][1]
		cmd := &Command{
			Name: CURVETO,
			Points: []Tuple{
				pdp.transform.Apply(Tuple{x1, y1}),
				pdp.transform.Apply(Tuple{x2, y2}),
				pdp.transform.Apply(tuples[1+2*j]),
			},
		}
		pdp.p.command <- cmd
	}
	return nil
}

func parseTCurveTo(pdp *pathDescriptionParser, abs bool) error {
	tuples, err := parseTuples(pdp)
	if err != nil {
		return err
	}
	for j := 0; j < len(tuples); j++ {
		if !abs {
			tuples[j][0] += pdp.x
			tuples[j][1] += pdp.y
		}
		end := tuples[j]
		c1 := reflection(pdp.x, pdp.y, pdp.x1, pdp.y1)
		x1 := pdp.x + float64(2)/float64(3)*(c1[0]-pdp.x)
		y1 := pdp.y + float64(2)/float64(3)*(c1[1]-pdp.y)
		x2 := end[0] + float64(2)/float64(3)*(c1[0]-end[0])
		y2 := end[1] + float64(2)/float64(3)*(c1[1]-end[1])
		pdp.x = tuples[j][0]
		pdp.y = tuples[j][1]
		pdp.x1 = c1[0]
		pdp.y1 = c1[1]
		cmd := &Command{
			Name: CURVETO,
			Points: []Tuple{
				pdp.transform.Apply(Tuple{x1, y1}),
				pdp.transform.Apply(Tuple{x2, y2}),
				pdp.transform.Apply(tuples[j]),
			},
		}
		pdp.p.command <- cmd
	}
	return nil
}

func parseSCurveTo(pdp *pathDescriptionParser, abs bool) error {
	tuples, err := parseTuples(pdp)
	if err != nil {
		return err
	}

	for j := 0; j < len(tuples)/2; j++ {
		if !abs {
			for i := 2 * j; i < 2+2*j; i++ {
				tuples[i][0] += pdp.x
				tuples[i][1] += pdp.y
			}
		}
		c2 := reflection(pdp.x, pdp.y, tuples[0+2*j][0], tuples[0+2*j][1])
		pdp.x = tuples[1+2*j][0]
		pdp.y = tuples[1+2*j][1]
		pdp.x2 = c2[0]
		pdp.y2 = c2[1]
		cmd := &Command{
			Name: CURVETO,
			Points: []Tuple{
				pdp.transform.Apply(c2),
				pdp.transform.Apply(tuples[0+2*j]),
				pdp.transform.Apply(tuples[1+2*j]),
			},
		}
		pdp.p.command <- cmd
	}
	return nil
}

func parseCurveTo(pdp *pathDescriptionParser, abs bool) error {
	tuples, err := parseTuples(pdp)
	if err != nil {
		return err
	}
	for j := 0; j < len(tuples)/3; j++ {
		if !abs {
			for i := 3 * j; i < 3+3*j; i++ {
				tuples[i][0] += pdp.x
				tuples[i][1] += pdp.y
			}
		}
		pdp.x = tuples[2+3*j][0]
		pdp.y = tuples[2+3*j][1]
		pdp.x2 = tuples[1+3*j][0]
		pdp.y2 = tuples[1+3*j][1]

		cmd := &Command{
			Name: CURVETO,
			Points: []Tuple{
				pdp.transform.Apply(tuples[0+3*j]),
				pdp.transform.Apply(tuples[1+3*j]),
				pdp.transform.Apply(tuples[2+3*j]),
			},
		}
		pdp.p.command <- cmd
	}
	return nil
}

func (p *Path) parseStyle() {
	if p.Style == "" {
		return
	}
	p.properties = splitStyle(p.Style)
	for key, val := range p.properties {
		switch key {
		case "stroke-width":
			sw, ok := strconv.ParseFloat(val, 64)
			if ok == nil {
				p.strokeWidth = sw
			}

		}
	}
}
