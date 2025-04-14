package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nurlyy/task_manager/internal/domain"
	"github.com/nurlyy/task_manager/pkg/logger"
	"github.com/segmentio/kafka-go"
)

// KafkaProducer реализует интерфейс продюсера для отправки сообщений в Kafka
type KafkaProducer struct {
	writer *kafka.Writer
	topics map[string]string
	logger logger.Logger
}

// NewKafkaProducer создает новый экземпляр KafkaProducer
func NewKafkaProducer(brokers []string, topics map[string]string, logger logger.Logger) *KafkaProducer {
	writer := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequireAll,
		MaxAttempts:  5,
		BatchSize:    100,
		BatchTimeout: 10 * time.Millisecond,
		Logger:       wrapLogger{log: logger}, // inject logger wrapper
	}

	return &KafkaProducer{
		writer: writer,
		topics: topics,
		logger: logger,
	}
}

// Close закрывает соединение с Kafka
func (p *KafkaProducer) Close() error {
	p.logger.Info("Closing Kafka producer")
	return p.writer.Close()
}

// PublishTaskCreated публикует событие о создании задачи
func (p *KafkaProducer) PublishTaskCreated(ctx context.Context, task *TaskEvent) error {
	event := TaskEvent{
		ID:          task.ID,
		Title:       task.Title,
		Description: task.Description,
		ProjectID:   task.ProjectID,
		Status:      string(task.Status),
		Priority:    string(task.Priority),
		AssigneeID:  task.AssigneeID,
		CreatedBy:   task.CreatedBy,
		DueDate:     task.DueDate,
		CreatedAt:   task.CreatedAt,
		UpdatedAt:   task.UpdatedAt,
		Type:        EventTypeTaskCreated,
	}

	return p.publishEvent(ctx, p.topics["task_created"], task.ID, event)
}

// PublishTaskUpdated публикует событие об обновлении задачи
func (p *KafkaProducer) PublishTaskUpdated(ctx context.Context, task *TaskEvent, changes map[string]interface{}) error {
	event := TaskEvent{
		ID:         task.ID,
		Title:      task.Title,
		ProjectID:  task.ProjectID,
		Status:     string(task.Status),
		Priority:   string(task.Priority),
		AssigneeID: task.AssigneeID,
		UpdatedAt:  task.UpdatedAt,
		Type:       EventTypeTaskUpdated,
		Changes:    changes,
	}

	return p.publishEvent(ctx, p.topics["task_updated"], task.ID, event)
}

// PublishTaskAssigned публикует событие о назначении задачи
func (p *KafkaProducer) PublishTaskAssigned(ctx context.Context, task *domain.Task, assignerID string) error {
	event := TaskEvent{
		ID:         task.ID,
		Title:      task.Title,
		ProjectID:  task.ProjectID,
		Status:     string(task.Status),
		Priority:   string(task.Priority),
		AssigneeID: task.AssigneeID,
		UpdatedAt:  task.UpdatedAt,
		Type:       EventTypeTaskAssigned,
		AssignerID: assignerID,
	}

	return p.publishEvent(ctx, p.topics["task_assigned"], task.ID, event)
}

// PublishTaskCommented публикует событие о комментировании задачи
func (p *KafkaProducer) PublishTaskCommented(ctx context.Context, task *domain.Task, comment *CommentEvent) error {
	event := CommentEvent{
		TaskID:    task.ID,
		TaskTitle: task.Title,
		CommentID: comment.CommentID,
		UserID:    comment.UserID,
		Content:   comment.Content,
		CreatedAt: comment.CreatedAt,
		Type:      EventTypeTaskCommented,
	}

	return p.publishEvent(ctx, p.topics["task_commented"], comment.CommentID, event)
}

// PublishProjectCreated публикует событие о создании проекта
func (p *KafkaProducer) PublishProjectCreated(ctx context.Context, project *ProjectEvent) error {
	event := ProjectEvent{
		ID:          project.ID,
		Name:        project.Name,
		Description: project.Description,
		Status:      string(project.Status),
		CreatedBy:   project.CreatedBy,
		CreatedAt:   project.CreatedAt,
		UpdatedAt:   project.UpdatedAt,
		Type:        EventTypeProjectCreated,
	}

	return p.publishEvent(ctx, p.topics["project_created"], project.ID, event)
}

// PublishProjectUpdated публикует событие об обновлении проекта
func (p *KafkaProducer) PublishProjectUpdated(ctx context.Context, project *ProjectEvent, changes map[string]interface{}) error {
	event := ProjectEvent{
		ID:        project.ID,
		Name:      project.Name,
		Status:    string(project.Status),
		UpdatedAt: project.UpdatedAt,
		Type:      EventTypeProjectUpdated,
		Changes:   changes,
	}

	return p.publishEvent(ctx, p.topics["project_updated"], project.ID, event)
}

// PublishProjectMemberAdded публикует событие о добавлении участника в проект
func (p *KafkaProducer) PublishProjectMemberAdded(ctx context.Context, projectID, projectName string, member *ProjectMemberEvent) error {
	event := ProjectMemberEvent{
		ProjectID:   projectID,
		ProjectName: projectName,
		UserID:      member.UserID,
		Role:        string(member.Role),
		InvitedBy:   member.InvitedBy,
		JoinedAt:    member.JoinedAt,
		Type:        EventTypeProjectMemberAdded,
	}

	return p.publishEvent(ctx, p.topics["project_member_added"], fmt.Sprintf("%s-%s", projectID, member.UserID), event)
}

// PublishProjectMemberRemoved публикует событие об удалении участника проекта
func (p *KafkaProducer) PublishProjectMemberRemoved(ctx context.Context, member *ProjectMemberEvent, removedBy string) error {
	event := ProjectMemberEvent{
		ProjectID:   member.ProjectID,
		ProjectName: member.ProjectName,
		UserID:      member.UserID,
		Role:        member.Role,
		InvitedBy:   member.InvitedBy,
		JoinedAt:    member.JoinedAt,
		Type:        EventTypeProjectMemberRemoved,
	}

	return p.publishEvent(ctx, p.topics["project_member_removed"], member.UserID, event)
}

// PublishNotification публикует уведомление
func (p *KafkaProducer) PublishNotification(ctx context.Context, notification *NotificationEvent) error {
	return p.publishEvent(ctx, p.topics["notifications"], notification.EntityID, notification)
}

// Вспомогательный метод для публикации событий

func (p *KafkaProducer) publishEvent(ctx context.Context, topic, key string, event interface{}) error {
	value, err := json.Marshal(event)
	if err != nil {
		p.logger.Error("Failed to marshal event", err, map[string]interface{}{
			"topic": topic,
			"key":   key,
		})
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	p.writer.Topic = topic

	start := time.Now()
	err = p.writer.WriteMessages(ctx,
		kafka.Message{
			Key:   []byte(key),
			Value: value,
			Time:  time.Now(),
		},
	)
	elapsed := time.Since(start)

	if err != nil {
		p.logger.Error("Failed to publish event", err, map[string]interface{}{
			"topic":   topic,
			"key":     key,
			"elapsed": elapsed.String(),
		})
		return fmt.Errorf("failed to publish event: %w", err)
	}

	p.logger.Debug("Successfully published event", map[string]interface{}{
		"topic":   topic,
		"key":     key,
		"elapsed": elapsed.String(),
	})

	return nil
}
