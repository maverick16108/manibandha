-- Галерея: альбомы и фотографии. Предопределённый альбом «Главная» (is_home) питает лендинг.
CREATE TABLE IF NOT EXISTS gallery_albums (
  id          serial PRIMARY KEY,
  title       varchar(255) NOT NULL,
  description text,
  is_home     boolean NOT NULL DEFAULT false,
  sort_order  integer NOT NULL DEFAULT 0,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS gallery_photos (
  id         serial PRIMARY KEY,
  album_id   integer NOT NULL REFERENCES gallery_albums(id) ON DELETE CASCADE,
  url        varchar(500) NOT NULL,
  caption    varchar(500),
  sort_order integer NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS ix_gallery_photos_album ON gallery_photos(album_id);

-- предопределённый альбом «Главная страница»
INSERT INTO gallery_albums (title, description, is_home, sort_order)
  SELECT 'Главная страница', 'Фотографии в галерее на главной странице сайта', true, -1
  WHERE NOT EXISTS (SELECT 1 FROM gallery_albums WHERE is_home);

-- текущие 12 фото с лендинга → в этот альбом
INSERT INTO gallery_photos (album_id, url, sort_order)
  SELECT (SELECT id FROM gallery_albums WHERE is_home LIMIT 1),
         '/guru/gallery/' || lpad(g::text, 2, '0') || '.jpg', g
  FROM generate_series(1, 12) g
  WHERE NOT EXISTS (
    SELECT 1 FROM gallery_photos WHERE album_id = (SELECT id FROM gallery_albums WHERE is_home LIMIT 1)
  );
