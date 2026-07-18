--
-- PostgreSQL database dump
--


-- Dumped from database version 16.13
-- Dumped by pg_dump version 16.13

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: public; Type: SCHEMA; Schema: -; Owner: -
--

-- *not* creating schema, since initdb creates it


--
-- Name: SCHEMA public; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON SCHEMA public IS '';


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: app_settings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.app_settings (
    key character varying(64) NOT NULL,
    value character varying(255) NOT NULL
);


--
-- Name: chat_members; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.chat_members (
    id integer NOT NULL,
    chat_id integer NOT NULL,
    user_id integer NOT NULL,
    role character varying(16) DEFAULT 'member'::character varying NOT NULL,
    last_read_seq bigint DEFAULT '0'::bigint NOT NULL,
    joined_at timestamp with time zone DEFAULT now(),
    pinned boolean DEFAULT false NOT NULL
);


--
-- Name: chat_members_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.chat_members_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: chat_members_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.chat_members_id_seq OWNED BY public.chat_members.id;


--
-- Name: chat_message_reactions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.chat_message_reactions (
    id integer NOT NULL,
    message_id integer NOT NULL,
    user_id integer NOT NULL,
    emoji character varying(16) NOT NULL,
    created_at timestamp with time zone DEFAULT now()
);


--
-- Name: chat_message_reactions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.chat_message_reactions_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: chat_message_reactions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.chat_message_reactions_id_seq OWNED BY public.chat_message_reactions.id;


--
-- Name: chat_message_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.chat_message_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: chat_messages; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.chat_messages (
    id integer NOT NULL,
    chat_id integer NOT NULL,
    seq bigint NOT NULL,
    client_uuid character varying(64),
    author_id integer,
    body text NOT NULL,
    reply_to_id integer,
    created_at timestamp with time zone DEFAULT now(),
    edited_at timestamp with time zone,
    edit_count integer DEFAULT 0 NOT NULL,
    deleted boolean DEFAULT false NOT NULL,
    reply_quote text
);


--
-- Name: chat_messages_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.chat_messages_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: chat_messages_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.chat_messages_id_seq OWNED BY public.chat_messages.id;


--
-- Name: chats; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.chats (
    id integer NOT NULL,
    type character varying(20) NOT NULL,
    title character varying(255),
    photo_url character varying(512),
    created_by integer,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now()
);


--
-- Name: chats_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.chats_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: chats_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.chats_id_seq OWNED BY public.chats.id;


--
-- Name: checklist_items; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.checklist_items (
    id integer NOT NULL,
    disciple_id integer NOT NULL,
    title character varying(512) NOT NULL,
    is_done boolean NOT NULL,
    note text,
    target character varying(11) NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: checklist_items_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.checklist_items_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: checklist_items_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.checklist_items_id_seq OWNED BY public.checklist_items.id;


--
-- Name: cities; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.cities (
    id integer NOT NULL,
    name character varying(120) NOT NULL,
    country character varying(120),
    region character varying(160)
);


--
-- Name: cities_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.cities_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: cities_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.cities_id_seq OWNED BY public.cities.id;


--
-- Name: conference_bans; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.conference_bans (
    id integer NOT NULL,
    conference_id integer NOT NULL,
    identity character varying(80) NOT NULL,
    name character varying(120),
    created_at timestamp with time zone DEFAULT now()
);


--
-- Name: conference_bans_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.conference_bans_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: conference_bans_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.conference_bans_id_seq OWNED BY public.conference_bans.id;


--
-- Name: conference_recordings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.conference_recordings (
    id integer NOT NULL,
    conference_id integer NOT NULL,
    egress_id character varying(80),
    filename character varying(255),
    status character varying(20) DEFAULT 'active'::character varying NOT NULL,
    duration_ms bigint DEFAULT '0'::bigint NOT NULL,
    size_bytes bigint DEFAULT '0'::bigint NOT NULL,
    started_at timestamp with time zone DEFAULT now(),
    ended_at timestamp with time zone,
    title character varying(255),
    description text
);


--
-- Name: conference_recordings_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.conference_recordings_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: conference_recordings_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.conference_recordings_id_seq OWNED BY public.conference_recordings.id;


--
-- Name: conferences; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.conferences (
    id integer NOT NULL,
    title character varying(255) NOT NULL,
    description text,
    mode character varying(20) DEFAULT 'interactive'::character varying NOT NULL,
    room character varying(80) NOT NULL,
    status character varying(20) DEFAULT 'scheduled'::character varying NOT NULL,
    host_id integer,
    scheduled_at timestamp with time zone,
    started_at timestamp with time zone,
    ended_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now(),
    mic_allowed boolean DEFAULT true NOT NULL,
    cam_allowed boolean DEFAULT true NOT NULL,
    guests_allowed boolean DEFAULT false NOT NULL,
    screen_allowed boolean DEFAULT true NOT NULL,
    auto_record boolean DEFAULT false NOT NULL,
    code character varying(16)
);


--
-- Name: conferences_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.conferences_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: conferences_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.conferences_id_seq OWNED BY public.conferences.id;


--
-- Name: countries; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.countries (
    id integer NOT NULL,
    name character varying(120) NOT NULL
);


--
-- Name: countries_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.countries_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: countries_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.countries_id_seq OWNED BY public.countries.id;


--
-- Name: disciple_files; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.disciple_files (
    id integer NOT NULL,
    disciple_id integer NOT NULL,
    uploaded_by integer,
    name character varying(255) NOT NULL,
    url character varying(500) NOT NULL,
    size integer,
    content_type character varying(120),
    created_at timestamp with time zone DEFAULT now()
);


--
-- Name: disciple_files_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.disciple_files_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: disciple_files_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.disciple_files_id_seq OWNED BY public.disciple_files.id;


--
-- Name: disciple_notes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.disciple_notes (
    id integer NOT NULL,
    disciple_id integer NOT NULL,
    author_id integer,
    text text NOT NULL,
    created_at timestamp with time zone DEFAULT now()
);


--
-- Name: disciple_notes_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.disciple_notes_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: disciple_notes_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.disciple_notes_id_seq OWNED BY public.disciple_notes.id;


--
-- Name: disciples; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.disciples (
    id integer NOT NULL,
    spiritual_name character varying(255),
    material_name character varying(255) NOT NULL,
    photo_url character varying(512),
    phone character varying(64),
    email character varying(255),
    messenger character varying(255),
    country character varying(120),
    city character varying(120),
    temple_id integer,
    marital_status character varying(11),
    date_of_birth date,
    initiation_status character varying(11) NOT NULL,
    harinama_date date,
    harinama_name character varying(255),
    brahman_date date,
    seva text,
    current_activity text,
    mentor_id integer,
    recommended_by character varying(255),
    application_date date,
    ready_for_initiation boolean NOT NULL,
    notes text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    region character varying(160),
    pranama_date date,
    ready_for_pranama boolean DEFAULT false NOT NULL,
    is_mentor boolean DEFAULT false NOT NULL,
    is_approved boolean DEFAULT true NOT NULL,
    mentor_name character varying(255),
    gender character varying(20)
);


--
-- Name: disciples_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.disciples_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: disciples_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.disciples_id_seq OWNED BY public.disciples.id;


--
-- Name: drafts; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drafts (
    id integer NOT NULL,
    user_id integer NOT NULL,
    scope character varying(64) NOT NULL,
    body text NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: drafts_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.drafts_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: drafts_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.drafts_id_seq OWNED BY public.drafts.id;


--
-- Name: events; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.events (
    id integer NOT NULL,
    title character varying(255) NOT NULL,
    location character varying(255),
    starts_on date NOT NULL,
    ends_on date,
    description text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: events_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.events_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: events_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.events_id_seq OWNED BY public.events.id;


--
-- Name: forum_post_likes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.forum_post_likes (
    id integer NOT NULL,
    post_id integer NOT NULL,
    user_id integer NOT NULL,
    created_at timestamp with time zone DEFAULT now(),
    emoji character varying(16) DEFAULT '❤️'::character varying NOT NULL
);


--
-- Name: forum_post_likes_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.forum_post_likes_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: forum_post_likes_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.forum_post_likes_id_seq OWNED BY public.forum_post_likes.id;


--
-- Name: forum_posts; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.forum_posts (
    id integer NOT NULL,
    topic_id integer NOT NULL,
    author_id integer,
    body text NOT NULL,
    created_at timestamp with time zone DEFAULT now(),
    edited_at timestamp with time zone,
    edit_count integer DEFAULT 0 NOT NULL
);


--
-- Name: forum_posts_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.forum_posts_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: forum_posts_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.forum_posts_id_seq OWNED BY public.forum_posts.id;


--
-- Name: forum_sections; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.forum_sections (
    id integer NOT NULL,
    title character varying(160) NOT NULL,
    description character varying(500),
    color character varying(16) DEFAULT '#c8742a'::character varying NOT NULL,
    author_id integer,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    cover_url character varying(500)
);


--
-- Name: forum_sections_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.forum_sections_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: forum_sections_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.forum_sections_id_seq OWNED BY public.forum_sections.id;


--
-- Name: forum_topic_reads; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.forum_topic_reads (
    id integer NOT NULL,
    topic_id integer NOT NULL,
    user_id integer NOT NULL,
    last_seen_at timestamp with time zone DEFAULT now()
);


--
-- Name: forum_topic_reads_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.forum_topic_reads_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: forum_topic_reads_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.forum_topic_reads_id_seq OWNED BY public.forum_topic_reads.id;


--
-- Name: forum_topics; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.forum_topics (
    id integer NOT NULL,
    title character varying(255) NOT NULL,
    author_id integer,
    pinned boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    section_id integer,
    views integer DEFAULT 0 NOT NULL,
    cover_url character varying(500)
);


--
-- Name: forum_topics_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.forum_topics_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: forum_topics_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.forum_topics_id_seq OWNED BY public.forum_topics.id;


--
-- Name: message_likes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.message_likes (
    id integer NOT NULL,
    message_id integer NOT NULL,
    user_id integer NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    emoji character varying(16) DEFAULT '❤️'::character varying NOT NULL
);


--
-- Name: message_likes_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.message_likes_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: message_likes_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.message_likes_id_seq OWNED BY public.message_likes.id;


--
-- Name: regions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.regions (
    id integer NOT NULL,
    name character varying(160) NOT NULL
);


--
-- Name: regions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.regions_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: regions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.regions_id_seq OWNED BY public.regions.id;


--
-- Name: roles; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.roles (
    id integer NOT NULL,
    key character varying(64) NOT NULL,
    name character varying(120) NOT NULL,
    is_system boolean DEFAULT false NOT NULL,
    is_superadmin boolean DEFAULT false NOT NULL,
    is_default boolean DEFAULT false NOT NULL,
    capabilities json DEFAULT '[]'::json NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: roles_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.roles_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: roles_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.roles_id_seq OWNED BY public.roles.id;


--
-- Name: sms_codes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sms_codes (
    id integer NOT NULL,
    phone character varying(20) NOT NULL,
    code character varying(8) NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    attempts integer DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: sms_codes_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.sms_codes_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: sms_codes_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.sms_codes_id_seq OWNED BY public.sms_codes.id;


--
-- Name: temples; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.temples (
    id integer NOT NULL,
    name character varying(255) NOT NULL,
    city character varying(120),
    country character varying(120),
    president_name character varying(255),
    notes text
);


--
-- Name: temples_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.temples_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: temples_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.temples_id_seq OWNED BY public.temples.id;


--
-- Name: thread_messages; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.thread_messages (
    id integer NOT NULL,
    thread_id integer NOT NULL,
    author_id integer,
    body character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    edited_at timestamp with time zone,
    edit_count integer DEFAULT 0 NOT NULL,
    reply_to_id integer
);


--
-- Name: thread_messages_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.thread_messages_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: thread_messages_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.thread_messages_id_seq OWNED BY public.thread_messages.id;


--
-- Name: thread_reads; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.thread_reads (
    id integer NOT NULL,
    thread_id integer NOT NULL,
    user_id integer NOT NULL,
    last_seen_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: thread_reads_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.thread_reads_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: thread_reads_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.thread_reads_id_seq OWNED BY public.thread_reads.id;


--
-- Name: threads; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.threads (
    id integer NOT NULL,
    kind character varying(8) NOT NULL,
    disciple_id integer NOT NULL,
    subject character varying(255),
    period character varying(7),
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    staff_seen_at timestamp with time zone
);


--
-- Name: threads_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.threads_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: threads_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.threads_id_seq OWNED BY public.threads.id;


--
-- Name: user_roles; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_roles (
    id integer NOT NULL,
    user_id integer NOT NULL,
    role_id integer NOT NULL
);


--
-- Name: user_roles_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.user_roles_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: user_roles_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.user_roles_id_seq OWNED BY public.user_roles.id;


--
-- Name: users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.users (
    id integer NOT NULL,
    email character varying(255) NOT NULL,
    hashed_password character varying(255) NOT NULL,
    full_name character varying(255) NOT NULL,
    role character varying(9) NOT NULL,
    is_active boolean NOT NULL,
    disciple_id integer,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    avatar_url character varying(500),
    phone character varying(20)
);


--
-- Name: users_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.users_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: users_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.users_id_seq OWNED BY public.users.id;


--
-- Name: chat_members id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_members ALTER COLUMN id SET DEFAULT nextval('public.chat_members_id_seq'::regclass);


--
-- Name: chat_message_reactions id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_message_reactions ALTER COLUMN id SET DEFAULT nextval('public.chat_message_reactions_id_seq'::regclass);


--
-- Name: chat_messages id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_messages ALTER COLUMN id SET DEFAULT nextval('public.chat_messages_id_seq'::regclass);


--
-- Name: chats id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chats ALTER COLUMN id SET DEFAULT nextval('public.chats_id_seq'::regclass);


--
-- Name: checklist_items id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.checklist_items ALTER COLUMN id SET DEFAULT nextval('public.checklist_items_id_seq'::regclass);


--
-- Name: cities id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cities ALTER COLUMN id SET DEFAULT nextval('public.cities_id_seq'::regclass);


--
-- Name: conference_bans id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.conference_bans ALTER COLUMN id SET DEFAULT nextval('public.conference_bans_id_seq'::regclass);


--
-- Name: conference_recordings id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.conference_recordings ALTER COLUMN id SET DEFAULT nextval('public.conference_recordings_id_seq'::regclass);


--
-- Name: conferences id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.conferences ALTER COLUMN id SET DEFAULT nextval('public.conferences_id_seq'::regclass);


--
-- Name: countries id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.countries ALTER COLUMN id SET DEFAULT nextval('public.countries_id_seq'::regclass);


--
-- Name: disciple_files id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.disciple_files ALTER COLUMN id SET DEFAULT nextval('public.disciple_files_id_seq'::regclass);


--
-- Name: disciple_notes id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.disciple_notes ALTER COLUMN id SET DEFAULT nextval('public.disciple_notes_id_seq'::regclass);


--
-- Name: disciples id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.disciples ALTER COLUMN id SET DEFAULT nextval('public.disciples_id_seq'::regclass);


--
-- Name: drafts id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drafts ALTER COLUMN id SET DEFAULT nextval('public.drafts_id_seq'::regclass);


--
-- Name: events id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.events ALTER COLUMN id SET DEFAULT nextval('public.events_id_seq'::regclass);


--
-- Name: forum_post_likes id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.forum_post_likes ALTER COLUMN id SET DEFAULT nextval('public.forum_post_likes_id_seq'::regclass);


--
-- Name: forum_posts id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.forum_posts ALTER COLUMN id SET DEFAULT nextval('public.forum_posts_id_seq'::regclass);


--
-- Name: forum_sections id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.forum_sections ALTER COLUMN id SET DEFAULT nextval('public.forum_sections_id_seq'::regclass);


--
-- Name: forum_topic_reads id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.forum_topic_reads ALTER COLUMN id SET DEFAULT nextval('public.forum_topic_reads_id_seq'::regclass);


--
-- Name: forum_topics id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.forum_topics ALTER COLUMN id SET DEFAULT nextval('public.forum_topics_id_seq'::regclass);


--
-- Name: message_likes id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_likes ALTER COLUMN id SET DEFAULT nextval('public.message_likes_id_seq'::regclass);


--
-- Name: regions id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.regions ALTER COLUMN id SET DEFAULT nextval('public.regions_id_seq'::regclass);


--
-- Name: roles id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.roles ALTER COLUMN id SET DEFAULT nextval('public.roles_id_seq'::regclass);


--
-- Name: sms_codes id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sms_codes ALTER COLUMN id SET DEFAULT nextval('public.sms_codes_id_seq'::regclass);


--
-- Name: temples id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.temples ALTER COLUMN id SET DEFAULT nextval('public.temples_id_seq'::regclass);


--
-- Name: thread_messages id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.thread_messages ALTER COLUMN id SET DEFAULT nextval('public.thread_messages_id_seq'::regclass);


--
-- Name: thread_reads id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.thread_reads ALTER COLUMN id SET DEFAULT nextval('public.thread_reads_id_seq'::regclass);


--
-- Name: threads id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.threads ALTER COLUMN id SET DEFAULT nextval('public.threads_id_seq'::regclass);


--
-- Name: user_roles id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_roles ALTER COLUMN id SET DEFAULT nextval('public.user_roles_id_seq'::regclass);


--
-- Name: users id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users ALTER COLUMN id SET DEFAULT nextval('public.users_id_seq'::regclass);


--
-- Name: app_settings app_settings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.app_settings
    ADD CONSTRAINT app_settings_pkey PRIMARY KEY (key);


--
-- Name: chat_members chat_members_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_members
    ADD CONSTRAINT chat_members_pkey PRIMARY KEY (id);


--
-- Name: chat_message_reactions chat_message_reactions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_message_reactions
    ADD CONSTRAINT chat_message_reactions_pkey PRIMARY KEY (id);


--
-- Name: chat_messages chat_messages_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_messages
    ADD CONSTRAINT chat_messages_pkey PRIMARY KEY (id);


--
-- Name: chats chats_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chats
    ADD CONSTRAINT chats_pkey PRIMARY KEY (id);


--
-- Name: checklist_items checklist_items_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.checklist_items
    ADD CONSTRAINT checklist_items_pkey PRIMARY KEY (id);


--
-- Name: cities cities_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cities
    ADD CONSTRAINT cities_pkey PRIMARY KEY (id);


--
-- Name: conference_bans conference_bans_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.conference_bans
    ADD CONSTRAINT conference_bans_pkey PRIMARY KEY (id);


--
-- Name: conference_recordings conference_recordings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.conference_recordings
    ADD CONSTRAINT conference_recordings_pkey PRIMARY KEY (id);


--
-- Name: conferences conferences_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.conferences
    ADD CONSTRAINT conferences_pkey PRIMARY KEY (id);


--
-- Name: countries countries_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.countries
    ADD CONSTRAINT countries_pkey PRIMARY KEY (id);


--
-- Name: disciple_files disciple_files_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.disciple_files
    ADD CONSTRAINT disciple_files_pkey PRIMARY KEY (id);


--
-- Name: disciple_notes disciple_notes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.disciple_notes
    ADD CONSTRAINT disciple_notes_pkey PRIMARY KEY (id);


--
-- Name: disciples disciples_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.disciples
    ADD CONSTRAINT disciples_pkey PRIMARY KEY (id);


--
-- Name: drafts drafts_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drafts
    ADD CONSTRAINT drafts_pkey PRIMARY KEY (id);


--
-- Name: events events_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.events
    ADD CONSTRAINT events_pkey PRIMARY KEY (id);


--
-- Name: forum_post_likes forum_post_likes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.forum_post_likes
    ADD CONSTRAINT forum_post_likes_pkey PRIMARY KEY (id);


--
-- Name: forum_posts forum_posts_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.forum_posts
    ADD CONSTRAINT forum_posts_pkey PRIMARY KEY (id);


--
-- Name: forum_sections forum_sections_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.forum_sections
    ADD CONSTRAINT forum_sections_pkey PRIMARY KEY (id);


--
-- Name: forum_topic_reads forum_topic_reads_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.forum_topic_reads
    ADD CONSTRAINT forum_topic_reads_pkey PRIMARY KEY (id);


--
-- Name: forum_topics forum_topics_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.forum_topics
    ADD CONSTRAINT forum_topics_pkey PRIMARY KEY (id);


--
-- Name: message_likes message_likes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_likes
    ADD CONSTRAINT message_likes_pkey PRIMARY KEY (id);


--
-- Name: regions regions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.regions
    ADD CONSTRAINT regions_pkey PRIMARY KEY (id);


--
-- Name: roles roles_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.roles
    ADD CONSTRAINT roles_pkey PRIMARY KEY (id);


--
-- Name: sms_codes sms_codes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sms_codes
    ADD CONSTRAINT sms_codes_pkey PRIMARY KEY (id);


--
-- Name: temples temples_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.temples
    ADD CONSTRAINT temples_pkey PRIMARY KEY (id);


--
-- Name: thread_messages thread_messages_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.thread_messages
    ADD CONSTRAINT thread_messages_pkey PRIMARY KEY (id);


--
-- Name: thread_reads thread_reads_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.thread_reads
    ADD CONSTRAINT thread_reads_pkey PRIMARY KEY (id);


--
-- Name: threads threads_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.threads
    ADD CONSTRAINT threads_pkey PRIMARY KEY (id);


--
-- Name: chat_members uq_chat_member; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_members
    ADD CONSTRAINT uq_chat_member UNIQUE (chat_id, user_id);


--
-- Name: chat_messages uq_chat_message_seq_val; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_messages
    ADD CONSTRAINT uq_chat_message_seq_val UNIQUE (seq);


--
-- Name: chat_messages uq_chat_message_uuid; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_messages
    ADD CONSTRAINT uq_chat_message_uuid UNIQUE (chat_id, client_uuid);


--
-- Name: chat_message_reactions uq_chat_reaction; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_message_reactions
    ADD CONSTRAINT uq_chat_reaction UNIQUE (message_id, user_id);


--
-- Name: conference_bans uq_conf_ban; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.conference_bans
    ADD CONSTRAINT uq_conf_ban UNIQUE (conference_id, identity);


--
-- Name: conferences uq_conference_room; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.conferences
    ADD CONSTRAINT uq_conference_room UNIQUE (room);


--
-- Name: drafts uq_draft_user_scope; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drafts
    ADD CONSTRAINT uq_draft_user_scope UNIQUE (user_id, scope);


--
-- Name: forum_post_likes uq_forum_post_like; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.forum_post_likes
    ADD CONSTRAINT uq_forum_post_like UNIQUE (post_id, user_id);


--
-- Name: forum_topic_reads uq_forum_topic_read; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.forum_topic_reads
    ADD CONSTRAINT uq_forum_topic_read UNIQUE (topic_id, user_id);


--
-- Name: message_likes uq_message_like; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_likes
    ADD CONSTRAINT uq_message_like UNIQUE (message_id, user_id);


--
-- Name: roles uq_roles_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.roles
    ADD CONSTRAINT uq_roles_key UNIQUE (key);


--
-- Name: thread_reads uq_thread_read; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.thread_reads
    ADD CONSTRAINT uq_thread_read UNIQUE (thread_id, user_id);


--
-- Name: user_roles uq_user_role; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_roles
    ADD CONSTRAINT uq_user_role UNIQUE (user_id, role_id);


--
-- Name: user_roles user_roles_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_roles
    ADD CONSTRAINT user_roles_pkey PRIMARY KEY (id);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: ix_chat_members_chat_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_chat_members_chat_id ON public.chat_members USING btree (chat_id);


--
-- Name: ix_chat_members_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_chat_members_user_id ON public.chat_members USING btree (user_id);


--
-- Name: ix_chat_message_reactions_message_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_chat_message_reactions_message_id ON public.chat_message_reactions USING btree (message_id);


--
-- Name: ix_chat_messages_chat_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_chat_messages_chat_id ON public.chat_messages USING btree (chat_id);


--
-- Name: ix_chat_messages_seq; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_chat_messages_seq ON public.chat_messages USING btree (seq);


--
-- Name: ix_chats_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_chats_type ON public.chats USING btree (type);


--
-- Name: ix_checklist_items_disciple_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_checklist_items_disciple_id ON public.checklist_items USING btree (disciple_id);


--
-- Name: ix_cities_name; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ix_cities_name ON public.cities USING btree (name);


--
-- Name: ix_conference_bans_conference_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_conference_bans_conference_id ON public.conference_bans USING btree (conference_id);


--
-- Name: ix_conference_recordings_conference_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_conference_recordings_conference_id ON public.conference_recordings USING btree (conference_id);


--
-- Name: ix_conference_recordings_egress_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_conference_recordings_egress_id ON public.conference_recordings USING btree (egress_id);


--
-- Name: ix_conferences_code; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ix_conferences_code ON public.conferences USING btree (code);


--
-- Name: ix_conferences_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_conferences_status ON public.conferences USING btree (status);


--
-- Name: ix_countries_name; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ix_countries_name ON public.countries USING btree (name);


--
-- Name: ix_disciple_files_disciple_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_disciple_files_disciple_id ON public.disciple_files USING btree (disciple_id);


--
-- Name: ix_disciple_notes_disciple_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_disciple_notes_disciple_id ON public.disciple_notes USING btree (disciple_id);


--
-- Name: ix_disciples_country; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_disciples_country ON public.disciples USING btree (country);


--
-- Name: ix_disciples_initiation_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_disciples_initiation_status ON public.disciples USING btree (initiation_status);


--
-- Name: ix_disciples_is_approved; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_disciples_is_approved ON public.disciples USING btree (is_approved);


--
-- Name: ix_disciples_is_mentor; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_disciples_is_mentor ON public.disciples USING btree (is_mentor);


--
-- Name: ix_disciples_material_name; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_disciples_material_name ON public.disciples USING btree (material_name);


--
-- Name: ix_disciples_mentor_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_disciples_mentor_id ON public.disciples USING btree (mentor_id);


--
-- Name: ix_disciples_region; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_disciples_region ON public.disciples USING btree (region);


--
-- Name: ix_disciples_spiritual_name; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_disciples_spiritual_name ON public.disciples USING btree (spiritual_name);


--
-- Name: ix_drafts_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_drafts_user_id ON public.drafts USING btree (user_id);


--
-- Name: ix_events_starts_on; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_events_starts_on ON public.events USING btree (starts_on);


--
-- Name: ix_forum_post_likes_post_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_forum_post_likes_post_id ON public.forum_post_likes USING btree (post_id);


--
-- Name: ix_forum_posts_topic_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_forum_posts_topic_id ON public.forum_posts USING btree (topic_id);


--
-- Name: ix_forum_topic_reads_topic_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_forum_topic_reads_topic_id ON public.forum_topic_reads USING btree (topic_id);


--
-- Name: ix_forum_topic_reads_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_forum_topic_reads_user_id ON public.forum_topic_reads USING btree (user_id);


--
-- Name: ix_forum_topics_pinned; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_forum_topics_pinned ON public.forum_topics USING btree (pinned);


--
-- Name: ix_forum_topics_section_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_forum_topics_section_id ON public.forum_topics USING btree (section_id);


--
-- Name: ix_message_likes_message_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_message_likes_message_id ON public.message_likes USING btree (message_id);


--
-- Name: ix_regions_name; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ix_regions_name ON public.regions USING btree (name);


--
-- Name: ix_sms_codes_phone; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_sms_codes_phone ON public.sms_codes USING btree (phone);


--
-- Name: ix_thread_messages_thread_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_thread_messages_thread_id ON public.thread_messages USING btree (thread_id);


--
-- Name: ix_thread_reads_thread_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_thread_reads_thread_id ON public.thread_reads USING btree (thread_id);


--
-- Name: ix_thread_reads_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_thread_reads_user_id ON public.thread_reads USING btree (user_id);


--
-- Name: ix_threads_disciple_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_threads_disciple_id ON public.threads USING btree (disciple_id);


--
-- Name: ix_threads_kind; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_threads_kind ON public.threads USING btree (kind);


--
-- Name: ix_user_roles_role_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_user_roles_role_id ON public.user_roles USING btree (role_id);


--
-- Name: ix_user_roles_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_user_roles_user_id ON public.user_roles USING btree (user_id);


--
-- Name: ix_users_email; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ix_users_email ON public.users USING btree (email);


--
-- Name: ix_users_phone; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ix_users_phone ON public.users USING btree (phone);


--
-- Name: chat_members chat_members_chat_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_members
    ADD CONSTRAINT chat_members_chat_id_fkey FOREIGN KEY (chat_id) REFERENCES public.chats(id) ON DELETE CASCADE;


--
-- Name: chat_members chat_members_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_members
    ADD CONSTRAINT chat_members_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: chat_message_reactions chat_message_reactions_message_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_message_reactions
    ADD CONSTRAINT chat_message_reactions_message_id_fkey FOREIGN KEY (message_id) REFERENCES public.chat_messages(id) ON DELETE CASCADE;


--
-- Name: chat_message_reactions chat_message_reactions_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_message_reactions
    ADD CONSTRAINT chat_message_reactions_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: chat_messages chat_messages_author_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_messages
    ADD CONSTRAINT chat_messages_author_id_fkey FOREIGN KEY (author_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: chat_messages chat_messages_chat_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_messages
    ADD CONSTRAINT chat_messages_chat_id_fkey FOREIGN KEY (chat_id) REFERENCES public.chats(id) ON DELETE CASCADE;


--
-- Name: chat_messages chat_messages_reply_to_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_messages
    ADD CONSTRAINT chat_messages_reply_to_id_fkey FOREIGN KEY (reply_to_id) REFERENCES public.chat_messages(id) ON DELETE SET NULL;


--
-- Name: chats chats_created_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chats
    ADD CONSTRAINT chats_created_by_fkey FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: checklist_items checklist_items_disciple_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.checklist_items
    ADD CONSTRAINT checklist_items_disciple_id_fkey FOREIGN KEY (disciple_id) REFERENCES public.disciples(id) ON DELETE CASCADE;


--
-- Name: conference_bans conference_bans_conference_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.conference_bans
    ADD CONSTRAINT conference_bans_conference_id_fkey FOREIGN KEY (conference_id) REFERENCES public.conferences(id) ON DELETE CASCADE;


--
-- Name: conference_recordings conference_recordings_conference_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.conference_recordings
    ADD CONSTRAINT conference_recordings_conference_id_fkey FOREIGN KEY (conference_id) REFERENCES public.conferences(id) ON DELETE CASCADE;


--
-- Name: conferences conferences_host_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.conferences
    ADD CONSTRAINT conferences_host_id_fkey FOREIGN KEY (host_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: disciple_files disciple_files_disciple_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.disciple_files
    ADD CONSTRAINT disciple_files_disciple_id_fkey FOREIGN KEY (disciple_id) REFERENCES public.disciples(id) ON DELETE CASCADE;


--
-- Name: disciple_files disciple_files_uploaded_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.disciple_files
    ADD CONSTRAINT disciple_files_uploaded_by_fkey FOREIGN KEY (uploaded_by) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: disciple_notes disciple_notes_author_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.disciple_notes
    ADD CONSTRAINT disciple_notes_author_id_fkey FOREIGN KEY (author_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: disciple_notes disciple_notes_disciple_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.disciple_notes
    ADD CONSTRAINT disciple_notes_disciple_id_fkey FOREIGN KEY (disciple_id) REFERENCES public.disciples(id) ON DELETE CASCADE;


--
-- Name: disciples disciples_mentor_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.disciples
    ADD CONSTRAINT disciples_mentor_id_fkey FOREIGN KEY (mentor_id) REFERENCES public.disciples(id) ON DELETE SET NULL;


--
-- Name: disciples disciples_temple_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.disciples
    ADD CONSTRAINT disciples_temple_id_fkey FOREIGN KEY (temple_id) REFERENCES public.temples(id) ON DELETE SET NULL;


--
-- Name: drafts drafts_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drafts
    ADD CONSTRAINT drafts_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: thread_messages fk_thread_messages_reply_to; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.thread_messages
    ADD CONSTRAINT fk_thread_messages_reply_to FOREIGN KEY (reply_to_id) REFERENCES public.thread_messages(id) ON DELETE SET NULL;


--
-- Name: users fk_users_disciple_id; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT fk_users_disciple_id FOREIGN KEY (disciple_id) REFERENCES public.disciples(id) ON DELETE SET NULL;


--
-- Name: forum_post_likes forum_post_likes_post_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.forum_post_likes
    ADD CONSTRAINT forum_post_likes_post_id_fkey FOREIGN KEY (post_id) REFERENCES public.forum_posts(id) ON DELETE CASCADE;


--
-- Name: forum_post_likes forum_post_likes_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.forum_post_likes
    ADD CONSTRAINT forum_post_likes_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: forum_posts forum_posts_author_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.forum_posts
    ADD CONSTRAINT forum_posts_author_id_fkey FOREIGN KEY (author_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: forum_posts forum_posts_topic_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.forum_posts
    ADD CONSTRAINT forum_posts_topic_id_fkey FOREIGN KEY (topic_id) REFERENCES public.forum_topics(id) ON DELETE CASCADE;


--
-- Name: forum_sections forum_sections_author_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.forum_sections
    ADD CONSTRAINT forum_sections_author_id_fkey FOREIGN KEY (author_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: forum_topic_reads forum_topic_reads_topic_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.forum_topic_reads
    ADD CONSTRAINT forum_topic_reads_topic_id_fkey FOREIGN KEY (topic_id) REFERENCES public.forum_topics(id) ON DELETE CASCADE;


--
-- Name: forum_topic_reads forum_topic_reads_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.forum_topic_reads
    ADD CONSTRAINT forum_topic_reads_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: forum_topics forum_topics_author_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.forum_topics
    ADD CONSTRAINT forum_topics_author_id_fkey FOREIGN KEY (author_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: forum_topics forum_topics_section_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.forum_topics
    ADD CONSTRAINT forum_topics_section_id_fkey FOREIGN KEY (section_id) REFERENCES public.forum_sections(id) ON DELETE CASCADE;


--
-- Name: message_likes message_likes_message_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_likes
    ADD CONSTRAINT message_likes_message_id_fkey FOREIGN KEY (message_id) REFERENCES public.thread_messages(id) ON DELETE CASCADE;


--
-- Name: message_likes message_likes_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_likes
    ADD CONSTRAINT message_likes_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: thread_messages thread_messages_author_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.thread_messages
    ADD CONSTRAINT thread_messages_author_id_fkey FOREIGN KEY (author_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: thread_messages thread_messages_thread_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.thread_messages
    ADD CONSTRAINT thread_messages_thread_id_fkey FOREIGN KEY (thread_id) REFERENCES public.threads(id) ON DELETE CASCADE;


--
-- Name: thread_reads thread_reads_thread_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.thread_reads
    ADD CONSTRAINT thread_reads_thread_id_fkey FOREIGN KEY (thread_id) REFERENCES public.threads(id) ON DELETE CASCADE;


--
-- Name: thread_reads thread_reads_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.thread_reads
    ADD CONSTRAINT thread_reads_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: threads threads_disciple_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.threads
    ADD CONSTRAINT threads_disciple_id_fkey FOREIGN KEY (disciple_id) REFERENCES public.disciples(id) ON DELETE CASCADE;


--
-- Name: user_roles user_roles_role_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_roles
    ADD CONSTRAINT user_roles_role_id_fkey FOREIGN KEY (role_id) REFERENCES public.roles(id) ON DELETE CASCADE;


--
-- Name: user_roles user_roles_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_roles
    ADD CONSTRAINT user_roles_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- PostgreSQL database dump complete
--


