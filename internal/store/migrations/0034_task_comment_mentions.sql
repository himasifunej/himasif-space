-- @Sebutan di komentar tugas (0.4.6-rc.5, canvas 3747dbbac0d0 desain 4): a
-- comment names the people it tags, each told by Ace and the bell. Additive:
-- older servers neither write nor read the table, and their comments simply
-- tag no one.
create table task_comment_mentions (
    comment_id uuid not null references task_comments(id),
    user_id    uuid not null references users(id),
    primary key (comment_id, user_id)
);
create index task_comment_mentions_user on task_comment_mentions(user_id);
