-- migrations/001_init.sql
create extension if not exists vector;

create table if not exists memories (
  id          bigserial primary key,
  title       text        not null,
  content     text        not null,
  type        text        not null default 'manual',
  project     text        not null default 'personal',
  scope       text        not null default 'project',
  topic_key   text,
  embedding   vector(768) not null,
  created_at  timestamptz not null default now(),
  updated_at  timestamptz not null default now()
);

create table if not exists sessions (
  id          text        primary key,
  project     text        not null,
  summary     text,
  started_at  timestamptz not null default now(),
  ended_at    timestamptz
);

create unique index if not exists memories_topic_key_project_idx
  on memories (project, topic_key)
  where topic_key is not null;

create index if not exists memories_embedding_idx
  on memories using ivfflat (embedding vector_cosine_ops)
  with (lists = 100);

create index if not exists memories_project_idx on memories (project);

create or replace function search_memories(
  query_embedding vector(768),
  filter_project  text    default null,
  filter_scope    text    default null,
  match_count     int     default 10,
  min_similarity  float   default 0.3
)
returns table (
  id          bigint,
  title       text,
  content     text,
  type        text,
  project     text,
  scope       text,
  topic_key   text,
  similarity  float,
  created_at  timestamptz
)
language sql stable as $$
  select
    m.id, m.title, m.content, m.type,
    m.project, m.scope, m.topic_key,
    1 - (m.embedding <=> query_embedding) as similarity,
    m.created_at
  from memories m
  where
    (filter_project is null or m.project = filter_project)
    and (filter_scope is null or m.scope = filter_scope)
    and 1 - (m.embedding <=> query_embedding) > min_similarity
  order by m.embedding <=> query_embedding
  limit match_count;
$$;
