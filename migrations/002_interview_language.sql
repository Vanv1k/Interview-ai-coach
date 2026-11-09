alter table interview_sessions
    add column if not exists interview_language text not null default 'ru';
