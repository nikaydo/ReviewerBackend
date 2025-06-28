CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE users_added(
	uuid UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	login TEXT NOT NULL UNIQUE,
	password TEXT NOT NULL,
	refresh_token TEXT NOT NULL);
    
CREATE TABLE review_requests (
	uuidReview UUID PRIMARY KEY,
	title TEXT,
	request TEXT);

CREATE TABLE userSettings (
	uuid UUID NOT NULL,
	request TEXT,
	mainPromt TEXT,
	model TEXT);

CREATE TABLE reviews (
	uuidUniq UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	uuid UUID,
	request TEXT NOT NULL,
	answer TEXT,
	date TIMESTAMPTZ DEFAULT now(),
	model TEXT,
	favorite BOOLEAN,
	think TEXT);

CREATE TABLE custom_prompt (
	uuidUser UUID,
	uuidUniq UUID DEFAULT gen_random_uuid(),
	name TEXT,
	promt TEXT);
