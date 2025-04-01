-- Включаем расширение для работы с UUID
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Создаем перечисляемые типы
CREATE TYPE user_role AS ENUM ('admin', 'manager', 'developer', 'viewer');
CREATE TYPE project_status AS ENUM ('active', 'on_hold', 'completed', 'archived');
CREATE TYPE project_role AS ENUM ('owner', 'manager', 'member', 'viewer');
CREATE TYPE task_status AS ENUM ('new', 'in_progress', 'on_hold', 'review', 'completed', 'cancelled');
CREATE TYPE task_priority AS ENUM ('low', 'medium', 'high', 'critical');
CREATE TYPE notification_type AS ENUM (
    'task_assigned', 'task_updated', 'task_commented', 'task_due_soon', 
    'task_overdue', 'project_member_added', 'project_updated', 'digest'
);
CREATE TYPE notification_status AS ENUM ('unread', 'read', 'deleted');

-- Таблица пользователей
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email VARCHAR(255) NOT NULL UNIQUE,
    hashed_password VARCHAR(255) NOT NULL,
    first_name VARCHAR(100) NOT NULL,
    last_name VARCHAR(100) NOT NULL,
    role user_role NOT NULL DEFAULT 'developer',
    avatar VARCHAR(255),
    position VARCHAR(100),
    department VARCHAR(100),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    last_login_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Индексы для таблицы пользователей
CREATE INDEX idx_users_email ON users (email);
CREATE INDEX idx_users_role ON users (role);

-- Таблица проектов
CREATE TABLE projects (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(100) NOT NULL,
    description TEXT NOT NULL,
    status project_status NOT NULL DEFAULT 'active',
    created_by UUID NOT NULL REFERENCES users(id),
    start_date TIMESTAMP WITH TIME ZONE,
    end_date TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Индексы для таблицы проектов
CREATE INDEX idx_projects_status ON projects (status);
CREATE INDEX idx_projects_created_by ON projects (created_by);

-- Таблица участников проекта
CREATE TABLE project_members (
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role project_role NOT NULL DEFAULT 'member',
    joined_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    invited_by UUID NOT NULL REFERENCES users(id),
    PRIMARY KEY (project_id, user_id)
);

-- Индексы для таблицы участников проекта
CREATE INDEX idx_project_members_user_id ON project_members (user_id);

-- Таблица задач
CREATE TABLE tasks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    title VARCHAR(200) NOT NULL,
    description TEXT NOT NULL,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    status task_status NOT NULL DEFAULT 'new',
    priority task_priority NOT NULL DEFAULT 'medium',
    assignee_id UUID REFERENCES users(id) ON DELETE SET NULL,
    created_by UUID NOT NULL REFERENCES users(id),
    due_date TIMESTAMP WITH TIME ZONE,
    estimated_hours NUMERIC(6, 2),
    spent_hours NUMERIC(6, 2) DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMP WITH TIME ZONE
);

-- Индексы для таблицы задач
CREATE INDEX idx_tasks_project_id ON tasks (project_id);
CREATE INDEX idx_tasks_assignee_id ON tasks (assignee_id);
CREATE INDEX idx_tasks_status ON tasks (status);
CREATE INDEX idx_tasks_priority ON tasks (priority);
CREATE INDEX idx_tasks_due_date ON tasks (due_date);

-- Таблица тегов задач
CREATE TABLE task_tags (
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    tag VARCHAR(50) NOT NULL,
    PRIMARY KEY (task_id, tag)
);

-- Индекс для поиска по тегам
CREATE INDEX idx_task_tags_tag ON task_tags (tag);

-- Таблица истории изменений задач
CREATE TABLE task_history (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id),
    field VARCHAR(50) NOT NULL,
    old_value TEXT,
    new_value TEXT,
    changed_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Индексы для таблицы истории изменений
CREATE INDEX idx_task_history_task_id ON task_history (task_id);
CREATE INDEX idx_task_history_changed_at ON task_history (changed_at);

-- Таблица комментариев
CREATE TABLE comments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id),
    content TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Индексы для таблицы комментариев
CREATE INDEX idx_comments_task_id ON comments (task_id);
CREATE INDEX idx_comments_user_id ON comments (user_id);

-- Таблица логирования времени
CREATE TABLE time_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id),
    hours NUMERIC(6, 2) NOT NULL,
    description TEXT NOT NULL,
    logged_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    log_date TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Индексы для таблицы логирования времени
CREATE INDEX idx_time_logs_task_id ON time_logs (task_id);
CREATE INDEX idx_time_logs_user_id ON time_logs (user_id);
CREATE INDEX idx_time_logs_log_date ON time_logs (log_date);

-- Таблица уведомлений
CREATE TABLE notifications (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type notification_type NOT NULL,
    title VARCHAR(255) NOT NULL,
    content TEXT NOT NULL,
    status notification_status NOT NULL DEFAULT 'unread',
    entity_id UUID NOT NULL,
    entity_type VARCHAR(20) NOT NULL,
    meta_data JSONB,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    read_at TIMESTAMP WITH TIME ZONE
);

-- Индексы для таблицы уведомлений
CREATE INDEX idx_notifications_user_id ON notifications (user_id);
CREATE INDEX idx_notifications_status ON notifications (status);
CREATE INDEX idx_notifications_created_at ON notifications (created_at);
CREATE INDEX idx_notifications_entity_id ON notifications (entity_id);

-- Таблица настроек уведомлений пользователей
CREATE TABLE user_notification_settings (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    notification_type notification_type NOT NULL,
    email_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    web_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    telegram_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    PRIMARY KEY (user_id, notification_type)
);

-- Таблица для токенов обновления
CREATE TABLE refresh_tokens (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token VARCHAR(255) NOT NULL UNIQUE,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    revoked BOOLEAN NOT NULL DEFAULT FALSE
);

-- Индекс для поиска токенов
CREATE INDEX idx_refresh_tokens_token ON refresh_tokens (token);
CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens (user_id);

-- Триггерная функция для обновления updated_at
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Триггеры для автоматического обновления updated_at
CREATE TRIGGER update_users_updated_at
BEFORE UPDATE ON users
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_projects_updated_at
BEFORE UPDATE ON projects
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_tasks_updated_at
BEFORE UPDATE ON tasks
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_comments_updated_at
BEFORE UPDATE ON comments
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();

-- Функция для отметки задачи как завершенной
CREATE OR REPLACE FUNCTION set_task_completed()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.status = 'completed' AND (OLD.status != 'completed' OR OLD.status IS NULL) THEN
        NEW.completed_at = NOW();
    ELSIF NEW.status != 'completed' THEN
        NEW.completed_at = NULL;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Триггер для автоматического заполнения поля completed_at
CREATE TRIGGER set_task_completed_trigger
BEFORE UPDATE OR INSERT ON tasks
FOR EACH ROW
EXECUTE FUNCTION set_task_completed();

-- Функция для логирования изменений задачи
CREATE OR REPLACE FUNCTION log_task_changes()
RETURNS TRIGGER AS $$
BEGIN
    -- Изменение статуса
    IF NEW.status IS DISTINCT FROM OLD.status THEN
        INSERT INTO task_history (task_id, user_id, field, old_value, new_value)
        VALUES (NEW.id, current_setting('app.current_user_id')::UUID, 'status', OLD.status::TEXT, NEW.status::TEXT);
    END IF;
    
    -- Изменение приоритета
    IF NEW.priority IS DISTINCT FROM OLD.priority THEN
        INSERT INTO task_history (task_id, user_id, field, old_value, new_value)
        VALUES (NEW.id, current_setting('app.current_user_id')::UUID, 'priority', OLD.priority::TEXT, NEW.priority::TEXT);
    END IF;
    
    -- Изменение исполнителя
    IF NEW.assignee_id IS DISTINCT FROM OLD.assignee_id THEN
        INSERT INTO task_history (task_id, user_id, field, old_value, new_value)
        VALUES (NEW.id, current_setting('app.current_user_id')::UUID, 'assignee_id', OLD.assignee_id::TEXT, NEW.assignee_id::TEXT);
    END IF;
    
    -- Изменение срока выполнения
    IF NEW.due_date IS DISTINCT FROM OLD.due_date THEN
        INSERT INTO task_history (task_id, user_id, field, old_value, new_value)
        VALUES (NEW.id, current_setting('app.current_user_id')::UUID, 'due_date', OLD.due_date::TEXT, NEW.due_date::TEXT);
    END IF;
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Триггер для логирования изменений задачи
CREATE TRIGGER log_task_changes_trigger
AFTER UPDATE ON tasks
FOR EACH ROW
EXECUTE FUNCTION log_task_changes();

-- Индекс для полнотекстового поиска по задачам
CREATE INDEX idx_tasks_search ON tasks USING GIN (to_tsvector('russian', title || ' ' || description));

-- Создание администратора по умолчанию
INSERT INTO users (
    id, email, hashed_password, first_name, last_name, role, is_active, created_at, updated_at
) VALUES (
    uuid_generate_v4(),
    'admin@tasktracker.com',
    '$2a$10$qPRsKZavTy/1LU6CVmRrL.lmVh.XeK3.ZKFGzDpV6CK7NWz/Jz1Ri', -- password: admin123
    'Admin',
    'User',
    'admin',
    TRUE,
    NOW(),
    NOW()
);