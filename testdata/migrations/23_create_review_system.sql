-- 23_create_review_system.sql
create table reviews (
  id serial primary key,
  user_id integer references users(id),
  product_id integer references products(id),
  rating rating not null,
  title short_text,
  review_text text check (length(review_text) between 10 and 2000),
  helpful_votes positive_integer default 0,
  verified_purchase boolean default false,
  created_at timestamptz default now(),
  constraint rating_verification check (
    (verified_purchase = true and rating >= 1.0) or
    (verified_purchase = false and rating >= 2.0)
  )
);

create view product_ratings as
select 
  p.id,
  p.name,
  p.category,
  count(r.id) as review_count,
  avg(r.rating)::rating as average_rating,
  sum(r.helpful_votes)::positive_integer as total_helpful_votes
from products p
left join reviews r on p.id = r.product_id
group by p.id, p.name, p.category;

---- create above / drop below ----
drop view product_ratings;
drop table reviews;
