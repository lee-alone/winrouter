package app

import (
	"context"
	"errors"
	"time"

	"winrouter/internal/observability"
	"winrouter/internal/subscriptions"
)

func (a *App) ListSubscriptions() ([]subscriptions.Subscription, error) {
	manager, err := a.getSubscriptionManager()
	if err != nil {
		return nil, err
	}
	return manager.List(), nil
}

func (a *App) AddSubscription(input subscriptions.Input) (subscriptions.Subscription, error) {
	if a.isCoreRunning() {
		return subscriptions.Subscription{}, errors.New("cannot add subscription while core is running; please stop the core first")
	}
	manager, err := a.getSubscriptionManager()
	if err != nil {
		return subscriptions.Subscription{}, err
	}
	result, err := manager.Add(input)
	if err == nil {
		a.observations.Log(observability.LevelInfo, "proxy", "Subscription added", "", map[string]any{"subscription_id": result.ID, "host": result.Host})
	}
	return result, err
}

func (a *App) UpdateSubscription(input subscriptions.Input) (subscriptions.Subscription, error) {
	if a.isCoreRunning() {
		return subscriptions.Subscription{}, errors.New("cannot modify subscription while core is running; please stop the core first")
	}
	manager, err := a.getSubscriptionManager()
	if err != nil {
		return subscriptions.Subscription{}, err
	}
	result, err := manager.UpdateDefinition(input)
	if err == nil {
		a.observations.Log(observability.LevelInfo, "proxy", "Subscription definition updated", "", map[string]any{"subscription_id": result.ID, "host": result.Host})
	}
	return result, err
}

func (a *App) DeleteSubscription(id string) error {
	if a.isCoreRunning() {
		return errors.New("cannot delete subscription while core is running; please stop the core first")
	}
	manager, err := a.getSubscriptionManager()
	if err != nil {
		return err
	}
	if err := manager.Delete(id); err != nil {
		return err
	}
	a.observations.Log(observability.LevelInfo, "proxy", "Subscription deleted", "", map[string]any{"subscription_id": id})
	return nil
}

func (a *App) RefreshSubscription(id string) (subscriptions.Subscription, error) {
	if a.isCoreRunning() {
		return subscriptions.Subscription{}, errors.New("cannot refresh subscriptions while core is running; please stop the core first")
	}
	manager, err := a.getSubscriptionManager()
	if err != nil {
		return subscriptions.Subscription{}, err
	}
	ctx, cancel := context.WithTimeout(a.ctx, 20*time.Second)
	defer cancel()
	result, err := manager.Refresh(ctx, id)
	if err != nil {
		a.observations.Log(observability.LevelWarning, "proxy", "Subscription update rejected; previous nodes retained", newCorrelationID(), map[string]any{"subscription_id": id, "error": err.Error()})
		return subscriptions.Subscription{}, err
	}
	a.observations.Log(observability.LevelInfo, "proxy", "Subscription updated", "", map[string]any{"subscription_id": id, "node_count": result.NodeCount})
	return result, nil
}

func (a *App) getSubscriptionManager() (*subscriptions.Manager, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.subscriptionError != nil {
		return nil, a.subscriptionError
	}
	if a.subscriptions == nil {
		return nil, errors.New("subscription manager is not started")
	}
	return a.subscriptions, nil
}
