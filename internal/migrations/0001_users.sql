-- +goose Up
create table if not exists public.service_user
(
    id         serial not null primary key,
    name       varchar(100) not null,
    email        varchar(100) not null
);

-- +goose Down
drop table public.service_user;