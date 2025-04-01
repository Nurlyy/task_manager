-- Удаление триггеров
DROP TRIGGER IF EXISTS log_task_changes_trigger ON tasks;
DROP TRIGGER IF EXISTS set_task_completed_trigger ON tasks;
DROP TRIGGER IF EXISTS update_comments_updated_at ON comments;
DROP TRIGGER IF EXISTS update_tasks_updated_at ON tasks;
DROP TRIGGER IF EXISTS update_projects_updated_at ON projects;
DROP TRIGGER IF EXISTS update_users_updated_at ON users;

-- Удаление функций
DROP FUNCTION IF EXISTS log_task_changes();
DROP FUNCTION IF EXISTS set_task_completed();
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Удаление таблиц
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS user_notification_settings;
DROP TABLE IF EXISTS notifications;
DROP TABLE IF EXISTS time_logs;
DROP TABLE IF EXISTS comments;
DROP TABLE IF EXISTS task_history;
DROP TABLE IF EXISTS task_tags;
DROP TABLE IF EXISTS tasks;
DROP TABLE IF EXISTS project_members;
DROP TABLE IF EXISTS projects;
DROP TABLE IF EXISTS users;

-- Удаление перечисляемых типов
DROP TYPE IF EXISTS notification_status;
DROP TYPE IF EXISTS notification_type;
DROP TYPE IF EXISTS task_priority;
DROP TYPE IF EXISTS task_status;
DROP TYPE IF EXISTS project_role;
DROP TYPE IF EXISTS project_status;
DROP TYPE IF EXISTS user_role;