package client

import (
	"time"

	"github.com/rs/zerolog/log"
	"github.com/zyedidia/generic/queue"
	"github.com/unitoftime/ecs"

	"github.com/unitoftime/mmo"
	"github.com/unitoftime/mmo/serdes"
)

type LastUpdate struct {
	Time time.Time
}

func ClientPollNetworkSystem(networkChannel chan serdes.WorldUpdate,
	updateQueue *queue.Queue[serdes.WorldUpdate]) ecs.System {

	// Read everything from the channel and push it into the updateQueue
	sys := ecs.System{"PollNetwork", func(dt time.Duration) {
	MainLoop:
		for {
			select {
			case update := <-networkChannel:
				updateQueue.Enqueue(update)

			default:
				break MainLoop
			}
		}
	}}

	return sys
}

func ClientPullFromUpdateQueue(world *ecs.World, updateQueue *queue.Queue[serdes.WorldUpdate], playerData *mmo.PlayerData) ecs.System {
	// TODO! - dynamic based on connection
	targetQueueSize := 3

	var everyOther int

	// Read a single element from the update queue
	sys := ecs.System{"PullUpdateQueue", func(dt time.Duration) {
		everyOther = (everyOther + 1) % 4
		if everyOther != 0 {
			return // skip
		}

		// // TODO - keep track of size
		queueSize := 0
		updateQueue.Each(func(u serdes.WorldUpdate) {
			queueSize++
		})
		if queueSize > targetQueueSize {
			// log.Print("UpdateQueue Desynchronization (TooBig): ", queueSize, targetQueueSize)
			// We want the next tick to run a little bit faster.
			// TODO - Optimization Note: The bigger you make the addition, the faster it gets back to target length, but the more dramatic the entity needs to be sped up to interp that distance. I could also change the %4 to %8 to make the speedup even more minimal. Right now this doesn't seem to noticeable
			everyOther = (everyOther + 1) % 4
		}//  else if queueSize < targetQueueSize {
		// 	log.Print("UpdateQueue Desynchronization (TooSmall): ", queueSize, targetQueueSize)
		// 	everyOther = (everyOther + 3) % 4 // Go back one and rerun
		// 	return
		// }
		// log.Print("UpdateQueueSize: ", queueSize)

		if updateQueue.Empty() {
			log.Print("UpdateQueue is Empty!")
			return
		}

		update := updateQueue.Dequeue()

		// Update our playerData tick information
		playerData.SetTicks(update.Tick, update.PlayerTick)

		for id, compList := range update.WorldData {
			compList = append(compList, ecs.C(LastUpdate{time.Now()}))
			ecs.Write(world, id, compList...)
		}

		// Delete all the entities in the deleteList
		if update.Delete != nil {
			for _, id := range update.Delete {
				ecs.Delete(world, id)
			}
		}
	}}

	return sys
}
