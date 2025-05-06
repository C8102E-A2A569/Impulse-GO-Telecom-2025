package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Configuration struct {
	Laps        int    `json:"laps"`
	LapLen      int    `json:"lapLen"`
	PenaltyLen  int    `json:"penaltyLen"`
	FiringLines int    `json:"firingLines"`
	Start       string `json:"start"`
	StartDelta  string `json:"startDelta"`
}

type EventLog struct {
	Time         time.Time
	EventID      int
	CompetitorID int
	ExtraParams  string
}

type Competitor struct {
	ID                 int
	Status             string // "Finished", "NotFinished", "NotStarted", "Disqualified"
	RegisteredTime     time.Time
	PlannedStartTime   time.Time
	ActualStartTime    time.Time
	FinishTime         time.Time
	CurrentLap         int
	LapTimes           []time.Duration
	LapStartTimes      []time.Time
	PenaltyTimes       []time.Duration
	PenaltyStartTimes  []time.Time
	PenaltyEndTimes    []time.Time
	TotalPenaltyTime   time.Duration
	Hits               int
	Shots              int
	CurrentFiringRange int
	DNFReason          string
}

type LapStats struct {
	Time  string
	Speed float64
}

func (c *Competitor) calculateStats(config Configuration) ([]LapStats, LapStats) {
	lapStats := make([]LapStats, len(c.LapTimes))
	for i, lapTime := range c.LapTimes {
		speed := float64(config.LapLen) / lapTime.Seconds()
		lapStats[i] = LapStats{
			Time:  formatDuration(lapTime),
			Speed: speed,
		}
	}

	penaltyStats := LapStats{}
	if c.TotalPenaltyTime > 0 {
		penaltySpeed := float64(config.PenaltyLen) / c.TotalPenaltyTime.Seconds()
		penaltyStats = LapStats{
			Time:  formatDuration(c.TotalPenaltyTime),
			Speed: penaltySpeed,
		}
	}

	return lapStats, penaltyStats
}

func parseTime(timeStr string) (time.Time, error) {
	if !strings.HasPrefix(timeStr, "[") || !strings.HasSuffix(timeStr, "]") {
		return time.Time{}, fmt.Errorf("time string must be enclosed in square brackets: %s", timeStr)
	}

	timeStr = strings.Trim(timeStr, "[]")

	return time.Parse("15:04:05.000", timeStr)
}

func formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60
	milliseconds := int(d.Milliseconds()) % 1000

	return fmt.Sprintf("%02d:%02d:%02d.%03d", hours, minutes, seconds, milliseconds)
}

func parseEventLog(line string) (EventLog, error) {
	parts := strings.SplitN(line, "] ", 2)
	if len(parts) < 2 {
		return EventLog{}, fmt.Errorf("invalid event log format: %s", line)
	}

	timeStr := parts[0] + "]"
	eventTime, err := parseTime(timeStr)
	if err != nil {
		return EventLog{}, fmt.Errorf("invalid time format: %s", err)
	}

	eventText := parts[1]
	fields := strings.Fields(eventText)
	if len(fields) < 2 {
		return EventLog{}, fmt.Errorf("invalid event format: %s", eventText)
	}

	eventID, err := strconv.Atoi(fields[0])
	if err != nil {
		return EventLog{}, fmt.Errorf("invalid event ID: %s", fields[0])
	}

	competitorID, err := strconv.Atoi(fields[1])
	if err != nil {
		return EventLog{}, fmt.Errorf("invalid competitor ID: %s", fields[1])
	}

	extraParams := ""
	if len(fields) > 2 {
		extraParams = strings.Join(fields[2:], " ")
	}

	return EventLog{
		Time:         eventTime,
		EventID:      eventID,
		CompetitorID: competitorID,
		ExtraParams:  extraParams,
	}, nil
}

func processEvents(events []EventLog, config Configuration) map[int]*Competitor {
	competitors := make(map[int]*Competitor)

	_, _ = parseTime("[" + config.Start + "]")

	startDelta, _ := time.Parse("15:04:05.000", config.StartDelta)
	_ = time.Duration(startDelta.Hour())*time.Hour +
		time.Duration(startDelta.Minute())*time.Minute +
		time.Duration(startDelta.Second())*time.Second +
		time.Duration(startDelta.Nanosecond())

	for _, event := range events {
		competitorID := event.CompetitorID

		if _, exists := competitors[competitorID]; !exists {
			if event.EventID == 1 {
				competitors[competitorID] = &Competitor{
					ID:              competitorID,
					RegisteredTime:  event.Time,
					Status:          "NotStarted", // Default status
					LapTimes:        make([]time.Duration, 0),
					LapStartTimes:   make([]time.Time, 0),
					PenaltyTimes:    make([]time.Duration, 0),
					PenaltyEndTimes: make([]time.Time, 0),
					Shots:           0,
					Hits:            0,
				}
			} else {
				// Skip events for non-registered competitors
				continue
			}
		}

		competitor := competitors[competitorID]

		switch event.EventID {
		case 1: // Registration
			fmt.Printf("[%s] The competitor(%d) registered\n", formatTime(event.Time), competitorID)

		case 2: // Start time set by draw
			startTimeStr := event.ExtraParams
			plannedStartTime, _ := parseTime("[" + startTimeStr + "]")
			competitor.PlannedStartTime = plannedStartTime
			fmt.Printf("[%s] The start time for the competitor(%d) was set by a draw to %s\n",
				formatTime(event.Time), competitorID, startTimeStr)

		case 3: // Competitor on start line
			fmt.Printf("[%s] The competitor(%d) is on the start line\n", formatTime(event.Time), competitorID)

		case 4: // Competitor started
			competitor.ActualStartTime = event.Time
			competitor.CurrentLap = 1
			competitor.LapStartTimes = append(competitor.LapStartTimes, event.Time)
			competitor.Status = "Started"
			fmt.Printf("[%s] The competitor(%d) has started\n", formatTime(event.Time), competitorID)

			// Check if competitor started too late (outside their start window)
			// The start window is the planned start time + a small tolerance (usually a few seconds)
			// For this implementation, we'll use a 1-second tolerance
			if event.Time.After(competitor.PlannedStartTime.Add(1 * time.Second)) {
				competitor.Status = "Disqualified"
				fmt.Printf("[%s] The competitor(%d) is disqualified\n", formatTime(event.Time), competitorID)
				// Generate outgoing event for disqualification (Event ID 32)
				fmt.Printf("[%s] 32 %d\n", formatTime(event.Time), competitorID)
			}

		case 5: // Competitor on firing range
			firingRange, _ := strconv.Atoi(event.ExtraParams)
			competitor.CurrentFiringRange = firingRange
			fmt.Printf("[%s] The competitor(%d) is on the firing range(%s)\n",
				formatTime(event.Time), competitorID, event.ExtraParams)

		case 6: // Target hit
			_, _ = strconv.Atoi(event.ExtraParams)
			competitor.Hits++
			competitor.Shots++
			fmt.Printf("[%s] The target(%s) has been hit by competitor(%d)\n",
				formatTime(event.Time), event.ExtraParams, competitorID)

		case 7: // Competitor left firing range
			fmt.Printf("[%s] The competitor(%d) left the firing range\n", formatTime(event.Time), competitorID)

		case 8: // Competitor entered penalty laps
			competitor.PenaltyStartTimes = append(competitor.PenaltyStartTimes, event.Time)
			fmt.Printf("[%s] The competitor(%d) entered the penalty laps\n", formatTime(event.Time), competitorID)

		case 9: // Competitor left penalty laps
			if len(competitor.PenaltyStartTimes) > len(competitor.PenaltyEndTimes) {
				lastPenaltyStart := competitor.PenaltyStartTimes[len(competitor.PenaltyStartTimes)-1]
				penaltyTime := event.Time.Sub(lastPenaltyStart)
				competitor.PenaltyTimes = append(competitor.PenaltyTimes, penaltyTime)
				competitor.PenaltyEndTimes = append(competitor.PenaltyEndTimes, event.Time)
				competitor.TotalPenaltyTime += penaltyTime
			}
			fmt.Printf("[%s] The competitor(%d) left the penalty laps\n", formatTime(event.Time), competitorID)

		case 10: // Competitor ended main lap
			if len(competitor.LapStartTimes) > 0 {
				lastLapStart := competitor.LapStartTimes[len(competitor.LapStartTimes)-1]
				lapTime := event.Time.Sub(lastLapStart)
				competitor.LapTimes = append(competitor.LapTimes, lapTime)

				competitor.CurrentLap++
				if competitor.CurrentLap <= config.Laps {
					competitor.LapStartTimes = append(competitor.LapStartTimes, event.Time)
				} else {
					competitor.FinishTime = event.Time

					if competitor.Status != "Disqualified" {
						competitor.Status = "Finished"

						fmt.Printf("[%s] 33 %d\n", formatTime(event.Time), competitorID)
						fmt.Printf("[%s] The competitor(%d) has finished\n", formatTime(event.Time), competitorID)
					}
				}
			}
			fmt.Printf("[%s] The competitor(%d) ended the main lap\n", formatTime(event.Time), competitorID)

		case 11: // Competitor can't continue
			competitor.Status = "NotFinished"
			competitor.DNFReason = event.ExtraParams
			fmt.Printf("[%s] The competitor(%d) can`t continue: %s\n",
				formatTime(event.Time), competitorID, event.ExtraParams)
		}
	}

	for _, competitor := range competitors {
		if competitor.Status == "NotStarted" && !competitor.PlannedStartTime.IsZero() {

			if time.Now().After(competitor.PlannedStartTime.Add(1 * time.Second)) {
				competitor.Status = "Disqualified"
				fmt.Printf("[%s] The competitor(%d) is disqualified\n",
					formatTime(competitor.PlannedStartTime.Add(1*time.Second)), competitor.ID)

				fmt.Printf("[%s] 32 %d\n", formatTime(competitor.PlannedStartTime.Add(1*time.Second)), competitor.ID)
			}
		}
	}

	return competitors
}

func formatTime(t time.Time) string {
	return t.Format("15:04:05.000")
}

func generateReport(competitors map[int]*Competitor, config Configuration) {

	var sortedCompetitors []*Competitor
	for _, competitor := range competitors {
		sortedCompetitors = append(sortedCompetitors, competitor)
	}

	sort.Slice(sortedCompetitors, func(i, j int) bool {
		ci, cj := sortedCompetitors[i], sortedCompetitors[j]

		// Status priorities: Finished > NotFinished > Disqualified > NotStarted
		statusPriority := map[string]int{
			"Finished":     0,
			"NotFinished":  1,
			"Disqualified": 2,
			"NotStarted":   3,
		}

		if ci.Status == "Finished" && cj.Status == "Finished" {

			timeI := ci.FinishTime.Sub(ci.ActualStartTime)
			if ci.ActualStartTime.After(ci.PlannedStartTime) {
				timeI += ci.ActualStartTime.Sub(ci.PlannedStartTime)
			}

			timeJ := cj.FinishTime.Sub(cj.ActualStartTime)
			if cj.ActualStartTime.After(cj.PlannedStartTime) {
				timeJ += cj.ActualStartTime.Sub(cj.PlannedStartTime)
			}

			return timeI < timeJ
		}

		return statusPriority[ci.Status] < statusPriority[cj.Status]
	})

	fmt.Println("\nFinal Results:")
	for _, competitor := range sortedCompetitors {
		lapStats, penaltyStats := competitor.calculateStats(config)

		formattedLapStats := make([]string, 0)
		for i := 0; i < len(lapStats); i++ {
			formattedLapStats = append(formattedLapStats,
				fmt.Sprintf("{%s, %.3f}", lapStats[i].Time, lapStats[i].Speed))
		}

		for i := len(lapStats); i < config.Laps; i++ {
			formattedLapStats = append(formattedLapStats, "{,}")
		}

		formattedPenaltyStats := "{,}"
		if penaltyStats.Time != "" {
			formattedPenaltyStats = fmt.Sprintf("{%s, %.3f}", penaltyStats.Time, penaltyStats.Speed)
		}

		var statusStr string
		switch competitor.Status {
		case "Finished":

			totalTime := competitor.FinishTime.Sub(competitor.ActualStartTime)
			if competitor.ActualStartTime.After(competitor.PlannedStartTime) {
				totalTime += competitor.ActualStartTime.Sub(competitor.PlannedStartTime)
			}
			statusStr = formatDuration(totalTime)
		case "NotFinished":
			statusStr = "NotFinished"
		case "Disqualified":
			statusStr = "Disqualified"
		case "NotStarted":
			statusStr = "NotStarted"
		default:
			statusStr = competitor.Status
		}

		fmt.Printf("[%s] %d [%s] %s %d/%d\n",
			statusStr,
			competitor.ID,
			strings.Join(formattedLapStats, ", "),
			formattedPenaltyStats,
			competitor.Hits,
			competitor.Shots)
	}
}

func main() {
	configPath := "sunny_5_skiers/config.json"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	configFile, err := os.Open(configPath)
	if err != nil {
		fmt.Println("Error opening configuration file:", err)
		return
	}
	defer configFile.Close()

	var config Configuration
	decoder := json.NewDecoder(configFile)
	if err := decoder.Decode(&config); err != nil {
		fmt.Println("Error parsing configuration:", err)
		return
	}

	eventsPath := "sunny_5_skiers/events"
	if len(os.Args) > 2 {
		eventsPath = os.Args[2]
	}
	eventsFile, err := os.Open(eventsPath)
	if err != nil {
		fmt.Println("Error opening events file:", err)
		return
	}
	defer eventsFile.Close()
	scanner := bufio.NewScanner(eventsFile)

	var events []EventLog
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		event, err := parseEventLog(line)
		if err != nil {
			fmt.Println("Error parsing event:", err)
			continue
		}

		events = append(events, event)
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("Error reading events:", err)
		return
	}

	competitors := processEvents(events, config)

	generateReport(competitors, config)
}
