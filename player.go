package main

import (
	"fmt"
	"slices"
	"strings"
)

// PlayerState represents the current state of a player in a round
type PlayerState int

const (
	Active PlayerState = iota
	Stayed
	Busted
)

// PlayerType represents whether a player is human or computer
type PlayerType int

const (
	Human PlayerType = iota
	Computer
)

type PlayerInterface interface {
	AddCard(card *Card) error
	AddToTotalScore()
	Bust()
	CalculateRoundScore() int
	ChooseActionTarget(gameState *GameState, actionType ActionType) (PlayerInterface, error)
	ChoosePositiveActionTarget(gameState *GameState, actionType ActionType) (PlayerInterface, error)
	GetHand() []*Card
	GetHandSummary() string
	GetName() string
	GetPlayerIcon() string
	GetTotalScore() int
	HasCards() bool
	HasSecondChance() bool
	IsActive() bool
	MakeHitStayDecision(gameState *GameState) (bool, error)
	NumberOfNumberCards() int
	ResetForNewRound() []*Card
	ShowHand()
	Stay()
	UseSecondChance() *Card
}

// Player represents a game player
type BasePlayer struct {
	Name          string
	TotalScore    int
	NumberCards   []*Card
	ModifierCards []*Card
	ActionCards   []*Card
	State         PlayerState
	SecondChance  bool
}

func (p *BasePlayer) Init(name string) {
	p.Name = name
	p.NumberCards = make([]*Card, 0)
	p.ModifierCards = make([]*Card, 0)
	p.ActionCards = make([]*Card, 0)
	p.State = Active
}

func (p *BasePlayer) GetName() string {
	return p.Name
}

func (p *BasePlayer) HasSecondChance() bool {
	return p.SecondChance
}

func (p *BasePlayer) GetTotalScore() int {
	return p.TotalScore
}

// AddCard adds a card to the player's hand
func (p *BasePlayer) AddCard(card *Card) error {
	switch card.Type {
	case NumberCard:
		// Check for duplicate number cards (busting)
		for _, existing := range p.NumberCards {
			if existing.Value == card.Value {
				// Player busts unless they have a second chance
				if p.HasSecondChance() {
					return fmt.Errorf("duplicate_with_second_chance:%d", card.Value)
				}
				p.Bust()
				return fmt.Errorf("bust:%d", card.Value)
			}
		}
		p.NumberCards = append(p.NumberCards, card)

		// Check for Flip 7
		if len(p.NumberCards) == 7 {
			p.Stay()
			return fmt.Errorf("flip7")
		}

	case ModifierCard:
		p.ModifierCards = append(p.ModifierCards, card)

	case ActionCard:
		if card.Action == SecondChance {
			if p.HasSecondChance() {
				return fmt.Errorf("second_chance_duplicate")
			}
			p.SecondChance = true
		}
		p.ActionCards = append(p.ActionCards, card)
	}

	return nil
}

func (p *BasePlayer) GetHand() []*Card {
	return slices.Concat(p.NumberCards, p.ModifierCards, p.ActionCards)
}

func (p *BasePlayer) NumberOfNumberCards() int {
	return len(p.NumberCards)
}

// UseSecondChance uses the second chance card to avoid busting
func (p *BasePlayer) UseSecondChance() *Card {
	if !p.HasSecondChance() {
		panic("no second change card to use")
	}

	p.SecondChance = false

	// Remove second chance card
	for i, card := range p.ActionCards {
		if card.Action == SecondChance {
			p.ActionCards = append(p.ActionCards[:i], p.ActionCards[i+1:]...)
			return card
		}
	}

	panic("no second chance card to use")
}

// Stay makes the player stay and bank their points
func (p *BasePlayer) Stay() {
	if p.State == Active {
		p.State = Stayed
	} else {
		panic("stay when player is not active")
	}
}

func (p *BasePlayer) Bust() {
	if p.State == Active {
		p.State = Busted
	} else {
		panic("bust when player is not active")
	}
}

// CalculateRoundScore calculates the player's score for the current round
func (p *BasePlayer) CalculateRoundScore() int {
	if p.State == Busted {
		return 0
	}

	// Calculate base score from number cards
	numberTotal := 0
	for _, card := range p.NumberCards {
		numberTotal += card.Value
	}

	// Apply multiplier if present
	for _, card := range p.ModifierCards {
		if card.Modifier == Multiply2 {
			numberTotal *= 2
			break
		}
	}

	// Add modifier points
	modifierTotal := 0
	for _, card := range p.ModifierCards {
		if card.Modifier != Multiply2 {
			modifierTotal += card.GetPoints()
		}
	}

	total := numberTotal + modifierTotal

	// Add Flip 7 bonus
	if len(p.NumberCards) == 7 {
		total += 15
	}

	return total
}

// AddToTotalScore adds the round score to the total score
func (p *BasePlayer) AddToTotalScore() {
	p.TotalScore += p.CalculateRoundScore()
}

// ResetForNewRound resets the player's state for a new round
func (p *BasePlayer) ResetForNewRound() []*Card {
	discardedCards := p.GetHand()
	p.NumberCards = make([]*Card, 0)
	p.ModifierCards = make([]*Card, 0)
	p.ActionCards = make([]*Card, 0)
	p.State = Active
	p.SecondChance = false
	return discardedCards
}

// IsActive returns true if the player is still active in the current round
func (p *BasePlayer) IsActive() bool {
	return p.State == Active
}

// HasCards returns true if the player has any number cards
func (p *BasePlayer) HasCards() bool {
	return len(p.NumberCards) > 0
}

// ShowHand displays the player's current hand
func (p *BasePlayer) ShowHand() {
	fmt.Printf("%s:\n", p.Name)

	if len(p.NumberCards) == 0 && len(p.ModifierCards) == 0 {
		fmt.Println("   No cards")
		return
	}

	// Show number cards
	if len(p.NumberCards) > 0 {
		fmt.Print("   Numbers: ")
		for i, card := range p.NumberCards {
			if i > 0 {
				fmt.Print(" ")
			}
			fmt.Print(card.String())
		}
		fmt.Println()
	}

	// Show modifier cards
	if len(p.ModifierCards) > 0 {
		fmt.Print("   Modifiers: ")
		for i, card := range p.ModifierCards {
			if i > 0 {
				fmt.Print(" ")
			}
			fmt.Print(card.String())
		}
		fmt.Println()
	}

	// Show special status
	if p.HasSecondChance() {
		fmt.Println("   🆘 Has Second Chance")
	}

	// Show state
	switch p.State {
	case Stayed:
		fmt.Printf("   ✅ STAYED - Round Score: %d\n", p.CalculateRoundScore())
	case Busted:
		fmt.Println("   💥 BUSTED")
	}

	fmt.Println()
}

// GetHandSummary returns a compact summary of the player's hand
func (p *BasePlayer) GetHandSummary() string {
	if p.State == Busted {
		return "💥 BUSTED"
	}

	if len(p.NumberCards) == 0 && len(p.ModifierCards) == 0 {
		return "No cards"
	}

	parts := make([]string, 0)

	if len(p.NumberCards) > 0 {
		numbers := make([]string, len(p.NumberCards))
		for i, card := range p.NumberCards {
			numbers[i] = fmt.Sprintf("%d", card.Value)
		}
		parts = append(parts, strings.Join(numbers, ","))
	}

	if len(p.ModifierCards) > 0 {
		mods := make([]string, len(p.ModifierCards))
		for i, card := range p.ModifierCards {
			mods[i] = card.String()
		}
		parts = append(parts, strings.Join(mods, ","))
	}

	result := strings.Join(parts, " | ")

	if p.State == Stayed {
		result += fmt.Sprintf(" (STAYED: %d pts)", p.CalculateRoundScore())
	}

	return result
}
