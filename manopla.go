package main

import (
	"embed"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
)

type move struct {
	name  string
	front bool
	back  bool
	leg   bool

	// a probabilidade é um valor percentual inverso, para evitar
	// de precisar preenchê-la para cada `move`.
	// 0.0 significa que o movimento pode sempre aparecer, enquanto 1.0
	// significa que o movimento nunca poderá aparecer
	prob float64
}

type rule func(move) bool

type globalOptions struct {
	maxDistinct int
	seqSize     int
	onlyArm     bool
	onlyLeg     bool
	interval    time.Duration
}

type rulesOptions struct {
	isFront       bool
	previousMoves []move
	seqSize       int
}

//go:embed sound
var fs embed.FS

func main() {
	o := globalOptions{}
	flag.IntVar(&o.maxDistinct, "d", 0, "Número de movimentos distintos. 0 permite todos os movimentos")
	flag.IntVar(&o.seqSize, "n", 2, "Número de movimentos por série")
	flag.BoolVar(&o.onlyLeg, "ol", false, "Permitir apenas golpes de perna")
	flag.BoolVar(&o.onlyArm, "oa", false, "Permitir apenas golpes de braço")
	flag.DurationVar(&o.interval, "t", 1*time.Second, "Intervalo entre as séries")
	flag.Parse()
	rand.Seed(2)

	var possibleMoves = []move{
		{name: "jab", front: true, back: false},
		{name: "direto", front: false, back: true},
		{name: "cruza", front: true, back: true, prob: 0.3},
		{name: "chuta", front: true, back: true, leg: true},
		{name: "tip", front: true, back: true, leg: true, prob: 0.5},
		{name: "upper", front: true, back: true, prob: 0.3},
		{name: "cotovelo", front: true, back: true, prob: 0.5},
		{name: "joelho", front: true, back: true, leg: true, prob: 0.5},
	}

	possibleMoves = applyGlobalOptions(possibleMoves, o)

	sr := beep.SampleRate(48000)
	err := speaker.Init(sr, sr.N(time.Second/10))
	if err != nil {
		panic(err)
	}

	isFront := true

	finish := make(chan struct{})
	go func() {
		_, err := os.Stdin.Read([]byte{0})
		if err != nil {
			log.Println(err)
		}
		close(finish)
	}()

	for {

		moves := []move{}
		s := ""
		if isFront {
			s += "F:"
		} else {
			s += "T:"
		}

		for i := 0; i < o.seqSize; i++ {
			move := nextMove(possibleMoves, rulesOptions{
				previousMoves: moves,
				seqSize:       o.seqSize,
				isFront:       isFront,
			})
			moves = append(moves, move)
			isFront = !isFront
			s += " " + move.name
		}

		fmt.Println(s)

		for _, m := range moves {
			err := playAudio(m.name)
			if err != nil {
				panic(err)
			}
		}

		select {
		case <-finish:
			return
		case <-time.After(o.interval):
		}
	}
}

func applyGlobalOptions(moves []move, o globalOptions) []move {
	result := []move{}

	for _, m := range moves {
		if o.onlyLeg && !m.leg {
			continue
		}

		if o.onlyArm && m.leg {
			continue
		}

		if !o.onlyLeg && m.leg && (o.seqSize > 1 && o.seqSize < 4) {
			continue
		}

		result = append(result, m)
	}

	if o.maxDistinct != 0 && len(result) > o.maxDistinct {
		result = result[:o.maxDistinct]
	}

	return result
}

func nextMove(possibleMoves []move, o rulesOptions) move {
	rules := []rule{}
	rules = append(rules, probabilityRule)

	// somente movimentos permitidos para aquele lado, ex:
	// jab só na frente, direto só com o braço de trás
	if o.isFront {
		rules = append(rules, frontRule)
	} else {
		rules = append(rules, backRule)
	}

	hasLeg := false
	hasArm := false

	for _, m := range possibleMoves {
		if m.leg {
			hasLeg = true
		} else {
			hasArm = true
		}
	}

	if hasLeg && hasArm {
		// exigir perna somente para *ultimo* golpe quando forem 4 ou mais movimentos
		// caso contrário, proibir perna caso não seja uma série de 1 só movimento
		if o.seqSize >= 4 && len(o.previousMoves) == o.seqSize-1 {
			rules = append(rules, mustBeLegRule)
		} else if o.seqSize != 1 {
			rules = append(rules, mustNotBeLegRule)
		}
	}

	allowedMoves := applyRules(possibleMoves, rules)
	i := rand.Intn(len(allowedMoves))
	return allowedMoves[i]
}

func applyRules(possibleMoves []move, rules []rule) []move {
	allowedMoves := []move{}

	// filtra os movimentos de acordo com as regras
	for _, m := range possibleMoves {
		pass := true

		for _, r := range rules {
			if !r(m) {
				pass = false
				break
			}
		}

		if pass {
			allowedMoves = append(allowedMoves, m)
		}
	}

	return allowedMoves
}

func playAudio(name string) error {
	f, err := fs.Open("sound/" + name + ".mp3")
	if err != nil {
		return err
	}
	defer f.Close()

	streamer, _, err := mp3.Decode(f)
	if err != nil {
		return err
	}
	defer streamer.Close()

	done := make(chan struct{})

	speaker.Play(beep.Seq(streamer, beep.Callback(func() {
		done <- struct{}{}
	})))

	<-done

	return nil
}

// o `level` define quais movimentos irão aparecer durante o treino.
// `level` = 0 (padrão) significa que todos os movimentos poderão parecer.
// `level` = 1 significa que somente os dois primeiros movimentos irão aparecer (jab e direto).
// `level` = 2 significa que os 3 primeiros movimentos irão aparecer.
// etc

func probabilityRule(m move) bool {
	p := rand.Float64()
	return m.prob <= p
}

func frontRule(m move) bool {
	return m.front
}

func backRule(m move) bool {
	return m.back
}

func mustBeLegRule(m move) bool {
	return m.leg
}

func mustNotBeLegRule(m move) bool {
	return !m.leg
}
