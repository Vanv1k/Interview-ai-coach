create table if not exists users (
    id bigserial primary key,
    telegram_id bigint not null unique,
    username text not null default '',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create table if not exists specializations (
    id bigserial primary key,
    slug text not null unique,
    name text not null,
    category text not null,
    active boolean not null default true,
    created_at timestamptz not null default now()
);

create table if not exists interview_sessions (
    id bigserial primary key,
    user_id bigint not null references users(id),
    specialization_id bigint not null references specializations(id),
    level text not null check (level in ('junior', 'middle', 'senior')),
    mode text not null check (mode in ('quick', 'standard', 'full')),
    status text not null check (status in ('active', 'finished', 'stopped')),
    started_at timestamptz not null default now(),
    finished_at timestamptz,
    overall_score integer,
    grade text not null default '',
    interview_language text not null default 'ru',
    current_question text not null default '',
    current_topic text not null default '',
    current_difficulty text not null default 'same',
    question_count integer not null default 0,
    follow_up_count_for_topic integer not null default 0,
    plan_json jsonb not null default '[]'::jsonb,
    state_json jsonb not null default '{}'::jsonb,
    updated_at timestamptz not null default now()
);

create index if not exists idx_interview_sessions_user_status on interview_sessions(user_id, status);
create index if not exists idx_interview_sessions_started_at on interview_sessions(started_at desc);

create table if not exists interview_answers (
    id bigserial primary key,
    session_id bigint not null references interview_sessions(id) on delete cascade,
    question text not null,
    answer text not null,
    topic text not null,
    difficulty text not null,
    is_follow_up boolean not null default false,
    question_order integer not null,
    score integer not null,
    evaluation_json jsonb not null,
    created_at timestamptz not null default now(),
    unique(session_id, question_order)
);

create index if not exists idx_interview_answers_session_order on interview_answers(session_id, question_order);

insert into specializations (slug, name, category, active) values
    ('go-backend', 'Go Developer', 'backend', true),
    ('java-backend', 'Java Developer', 'backend', true),
    ('ts-frontend', 'JavaScript/TypeScript Developer', 'frontend', true)
on conflict (slug) do update set
    name = excluded.name,
    category = excluded.category,
    active = excluded.active;
