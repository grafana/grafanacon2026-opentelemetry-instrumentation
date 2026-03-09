-- Barcelona Tapas Finder — Database Schema + Seed Data

-- Users
CREATE TABLE users (
    id           VARCHAR(8)   PRIMARY KEY,
    username     VARCHAR(50)  NOT NULL UNIQUE,
    is_admin     BOOLEAN      NOT NULL DEFAULT false,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Restaurants
CREATE TABLE restaurants (
    id           VARCHAR(8)     PRIMARY KEY,
    slug         VARCHAR(200)   NOT NULL UNIQUE,
    name         VARCHAR(200)   NOT NULL,
    address      VARCHAR(300),
    neighborhood VARCHAR(100),
    description  TEXT,
    hours        JSONB          NOT NULL DEFAULT '[]',
    options      TEXT[]         NOT NULL DEFAULT '{}',
    tapas_menu   JSONB          NOT NULL DEFAULT '[]',
    avg_rating   DECIMAL(3,2)   NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

-- Photos (max 2 per restaurant, ordered by created_at)
CREATE TABLE photos (
    id            VARCHAR(8)   PRIMARY KEY,
    restaurant_id VARCHAR(8)   NOT NULL REFERENCES restaurants(id) ON DELETE CASCADE,
    data          BYTEA        NOT NULL,
    content_type  VARCHAR(50)  NOT NULL DEFAULT 'image/jpeg',
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Ratings
CREATE TABLE ratings (
    id            VARCHAR(8)   PRIMARY KEY,
    user_id       VARCHAR(8)   NOT NULL REFERENCES users(id),
    restaurant_id VARCHAR(8)   NOT NULL REFERENCES restaurants(id) ON DELETE CASCADE,
    rating        SMALLINT     NOT NULL CHECK (rating BETWEEN 1 AND 5),
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, restaurant_id)
);

-- Indexes
CREATE INDEX idx_restaurants_neighborhood ON restaurants(neighborhood);
CREATE INDEX idx_restaurants_avg_rating   ON restaurants(avg_rating DESC);
CREATE INDEX idx_ratings_restaurant       ON ratings(restaurant_id);
CREATE INDEX idx_ratings_user             ON ratings(user_id);
CREATE INDEX idx_photos_restaurant        ON photos(restaurant_id, created_at);

-- ============================================================
-- SEED DATA
-- ============================================================

-- Users
INSERT INTO users (id, username, is_admin) VALUES
    ('a1f3e2d4', 'admin',  true),
    ('b2c4d5e6', 'alice',  false),
    ('c3d5e6f7', 'bob',    false),
    ('d4e6f708', 'carla',  false),
    ('e5f70819', 'david',  false);

-- Restaurants
INSERT INTO restaurants (id, slug, name, address, neighborhood, description, hours, options, tapas_menu, avg_rating) VALUES

-- 1. El Rincon Dorado — Gràcia
(
    'f6081920',
    'el_rincon_dorado',
    'El Rincón Dorado',
    'Carrer de la Llibertat 14, Barcelona',
    'Gràcia',
    'A lively neighbourhood bar with stone walls, wooden beams, and an ever-changing selection of seasonal tapas. Beloved by locals for its generous portions and unhurried atmosphere.',
    '[{"day":"monday","open":"13:00","close":"23:00"},{"day":"tuesday","open":"13:00","close":"23:00"},{"day":"wednesday","open":"13:00","close":"23:00"},{"day":"thursday","open":"13:00","close":"23:30"},{"day":"friday","open":"13:00","close":"00:00"},{"day":"saturday","open":"12:00","close":"00:00"},{"day":"sunday","open":"12:00","close":"22:00"}]',
    '{"vegetarian"}',
    '[{"name":"Patatas Bravas","price":6.50,"options":["vegan","vegetarian"]},{"name":"Croquetas de Jamón","price":7.00,"options":[]},{"name":"Pan con Tomate","price":4.00,"options":["vegan","vegetarian"]},{"name":"Tortilla Española","price":8.00,"options":["vegetarian"]},{"name":"Pimientos de Padrón","price":6.00,"options":["vegan","vegetarian"]}]',
    4.3
),

-- 2. La Tasca del Dragón — El Born
(
    '07192a31',
    'la_tasca_del_dragon',
    'La Tasca del Dragón',
    'Carrer del Rec 27, Barcelona',
    'El Born',
    'Hidden down a narrow cobblestone lane, this intimate tapas den serves creative small plates that blend tradition with a playful twist. Dimly lit with exposed brick and mismatched vintage furniture.',
    '[{"day":"tuesday","open":"14:00","close":"23:00"},{"day":"wednesday","open":"14:00","close":"23:00"},{"day":"thursday","open":"14:00","close":"23:30"},{"day":"friday","open":"14:00","close":"00:30"},{"day":"saturday","open":"13:00","close":"01:00"},{"day":"sunday","open":"13:00","close":"22:00"}]',
    '{"vegan","vegetarian"}',
    '[{"name":"Gazpacho Shot","price":4.00,"options":["vegan","vegetarian"]},{"name":"Boquerones en Vinagre","price":7.50,"options":[]},{"name":"Champiñones al Ajillo","price":7.00,"options":["vegan","vegetarian"]},{"name":"Albóndigas Caseras","price":9.00,"options":[]},{"name":"Tabla de Quesos","price":12.00,"options":["vegetarian"]}]',
    4.7
),

-- 3. Casa Mirabel — Barceloneta
(
    '182a3b42',
    'casa_mirabel',
    'Casa Mirabel',
    'Passeig de Joan de Borbó 52, Barcelona',
    'Barceloneta',
    'A breezy seafront spot specialising in fresh-catch anchovy tapas and crispy fried seafood. The outdoor terrace fills up fast on summer evenings — arrive early or wait for the view.',
    '[{"day":"monday","open":"12:00","close":"22:30"},{"day":"tuesday","open":"12:00","close":"22:30"},{"day":"wednesday","open":"12:00","close":"22:30"},{"day":"thursday","open":"12:00","close":"23:00"},{"day":"friday","open":"12:00","close":"23:30"},{"day":"saturday","open":"11:30","close":"00:00"},{"day":"sunday","open":"11:30","close":"22:00"}]',
    '{}',
    '[{"name":"Anchoas del Cantábrico","price":11.00,"options":[]},{"name":"Gambas al Ajillo","price":13.00,"options":[]},{"name":"Calamares Fritos","price":9.50,"options":[]},{"name":"Mejillones en Escabeche","price":8.00,"options":[]},{"name":"Pan con Tomate","price":3.50,"options":["vegan","vegetarian"]}]',
    4.1
),

-- 4. Bar Les Tres Perles — Eixample
(
    '293b4c53',
    'bar_les_tres_perles',
    'Bar Les Tres Perles',
    'Carrer del Consell de Cent 215, Barcelona',
    'Eixample',
    'A polished modernist bar tucked between boutiques in the Eixample grid. Known for its elegant pintxos counter and a curated natural wine list. Smart-casual crowd, excellent vermouth.',
    '[{"day":"monday","open":"12:00","close":"16:00"},{"day":"tuesday","open":"12:00","close":"16:00"},{"day":"wednesday","open":"12:00","close":"16:00"},{"day":"thursday","open":"12:00","close":"16:00"},{"day":"thursday","open":"19:00","close":"23:00"},{"day":"friday","open":"12:00","close":"16:00"},{"day":"friday","open":"19:00","close":"23:30"},{"day":"saturday","open":"12:00","close":"23:30"},{"day":"sunday","open":"12:00","close":"18:00"}]',
    '{"vegetarian"}',
    '[{"name":"Pintxo de Bacallà","price":5.00,"options":[]},{"name":"Pintxo de Bolet i Brie","price":4.50,"options":["vegetarian"]},{"name":"Croquetes de Botifarra","price":6.50,"options":[]},{"name":"Tapa de Gamba Vermella","price":14.00,"options":[]},{"name":"Olives Marinades","price":3.50,"options":["vegan","vegetarian"]}]',
    3.9
),

-- 5. El Fumador Lento — Poble Sec
(
    '3a4c5d64',
    'el_fumador_lento',
    'El Fumador Lento',
    'Carrer de Blai 38, Barcelona',
    'Poble Sec',
    'A low-key pintxos bar on the famous Carrer de Blai. Smoky aromas, cheap drinks, and a packed bar counter overflowing with bread-topped bites. Cash only. No reservations. Absolutely worth it.',
    '[{"day":"tuesday","open":"18:00","close":"23:30"},{"day":"wednesday","open":"18:00","close":"23:30"},{"day":"thursday","open":"18:00","close":"00:00"},{"day":"friday","open":"18:00","close":"01:00"},{"day":"saturday","open":"13:00","close":"01:00"},{"day":"sunday","open":"13:00","close":"22:30"}]',
    '{}',
    '[{"name":"Pintxo de Morcilla","price":2.50,"options":[]},{"name":"Pintxo de Truita","price":2.50,"options":["vegetarian"]},{"name":"Pintxo de Salmó Fumat","price":3.00,"options":[]},{"name":"Pintxo de Pebrot i Anxova","price":2.50,"options":[]},{"name":"Cervesa de Barril","price":2.50,"options":["vegan","vegetarian"]}]',
    4.5
),

-- 6. La Bóveda del Gòtic — Barri Gòtic
(
    '4b5d6e75',
    'la_boveda_del_gotic',
    'La Bóveda del Gòtic',
    'Carrer dels Escudellers 17, Barcelona',
    'Barri Gòtic',
    'Set in a medieval stone vault steps from the cathedral, this historic bar serves traditional Catalan tapas alongside house vermouth and local craft beers. One of the oldest continuously operating bars in the neighbourhood.',
    '[{"day":"monday","open":"12:00","close":"23:00"},{"day":"tuesday","open":"12:00","close":"23:00"},{"day":"wednesday","open":"12:00","close":"23:00"},{"day":"thursday","open":"12:00","close":"23:30"},{"day":"friday","open":"12:00","close":"00:30"},{"day":"saturday","open":"11:00","close":"01:00"},{"day":"sunday","open":"11:00","close":"22:00"}]',
    '{"vegetarian"}',
    '[{"name":"Pa amb Tomàquet i Pernil","price":6.00,"options":[]},{"name":"Escalivada sobre Torrada","price":7.00,"options":["vegan","vegetarian"]},{"name":"Cargols a la Llauna","price":9.00,"options":[]},{"name":"Formatge Manxec amb Mel","price":8.50,"options":["vegetarian"]},{"name":"Bunyols de Bacallà","price":7.50,"options":[]}]',
    4.0
),

-- 7. Vermouth & Company — El Born
(
    '5c6e7f86',
    'vermouth_company',
    'Vermouth & Company',
    'Carrer del Parlament 39, Barcelona',
    'El Born',
    'A relaxed Sunday-afternoon kind of place that is somehow open every day. Three vermouths on tap, excellent house-marinated anchovies, and a rotating cast of regulars arguing about football.',
    '[{"day":"monday","open":"12:00","close":"21:00"},{"day":"tuesday","open":"12:00","close":"21:00"},{"day":"wednesday","open":"12:00","close":"21:00"},{"day":"thursday","open":"12:00","close":"22:00"},{"day":"friday","open":"12:00","close":"23:00"},{"day":"saturday","open":"11:00","close":"23:00"},{"day":"sunday","open":"11:00","close":"21:00"}]',
    '{"vegan","vegetarian"}',
    '[{"name":"Anxoves amb Olives","price":8.00,"options":[]},{"name":"Patates Fregides Artesanes","price":5.00,"options":["vegan","vegetarian"]},{"name":"Musclos al Vapor","price":9.00,"options":[]},{"name":"Pebrots Farcits de Formatge","price":7.50,"options":["vegetarian"]},{"name":"Hummus amb Pa de Pita","price":6.50,"options":["vegan","vegetarian"]}]',
    4.4
),

-- 8. Sorra i Sal — Barceloneta
(
    '6d7f8097',
    'sorra_i_sal',
    'Sorra i Sal',
    'Carrer de la Mar 9, Barcelona',
    'Barceloneta',
    'Tiny beachside bar with mismatched plastic chairs and a chalk-board menu that changes with the morning catch. The fried bacalà and cold beer combination is the stuff of local legend.',
    '[{"day":"wednesday","open":"12:00","close":"22:00"},{"day":"thursday","open":"12:00","close":"22:00"},{"day":"friday","open":"12:00","close":"23:00"},{"day":"saturday","open":"11:00","close":"23:30"},{"day":"sunday","open":"11:00","close":"21:00"}]',
    '{}',
    '[{"name":"Bunyols de Bacallà","price":7.00,"options":[]},{"name":"Tapa de Pop a la Gallega","price":11.00,"options":[]},{"name":"Escopinyes al Vapor","price":9.50,"options":[]},{"name":"Croquetes del Dia","price":6.50,"options":[]},{"name":"Pa amb Tomàquet","price":3.00,"options":["vegan","vegetarian"]}]',
    3.7
),

-- 9. La Huerta Verde — Gràcia
(
    '7e8091a8',
    'la_huerta_verde',
    'La Huerta Verde',
    'Carrer de Torrent de l''Olla 77, Barcelona',
    'Gràcia',
    'The neighbourhood''s favourite plant-based tapas bar. Every dish is vegan by default, with hearty portions inspired by traditional Catalan and Spanish recipes. Warm lighting, mismatched tiled floors.',
    '[{"day":"tuesday","open":"13:00","close":"22:30"},{"day":"wednesday","open":"13:00","close":"22:30"},{"day":"thursday","open":"13:00","close":"22:30"},{"day":"friday","open":"13:00","close":"23:00"},{"day":"saturday","open":"12:00","close":"23:00"},{"day":"sunday","open":"12:00","close":"21:30"}]',
    '{"vegan","vegetarian"}',
    '[{"name":"Croquetes de Cigró","price":7.00,"options":["vegan","vegetarian"]},{"name":"Albergínies Farcides","price":9.00,"options":["vegan","vegetarian"]},{"name":"Patates Braves Negres","price":6.50,"options":["vegan","vegetarian"]},{"name":"Pintxo de Tofu Fumat","price":5.50,"options":["vegan","vegetarian"]},{"name":"Torrada amb Crema de Bolets","price":7.50,"options":["vegan","vegetarian"]}]',
    4.6
),

-- 10. El Petit Mercat — Poble Sec
(
    '8f91a2b9',
    'el_petit_mercat',
    'El Petit Mercat',
    'Carrer de Tamarit 104, Barcelona',
    'Poble Sec',
    'A market-inspired wine bar where the counter doubles as a deli display. Imported cheeses, cured meats, and seasonal conserves share the menu with a thoughtful selection of Catalan and Aragonese wines.',
    '[{"day":"monday","open":"17:00","close":"22:30"},{"day":"tuesday","open":"17:00","close":"22:30"},{"day":"wednesday","open":"17:00","close":"22:30"},{"day":"thursday","open":"17:00","close":"23:00"},{"day":"friday","open":"16:00","close":"23:30"},{"day":"saturday","open":"12:00","close":"23:30"},{"day":"sunday","open":"12:00","close":"21:00"}]',
    '{"vegetarian"}',
    '[{"name":"Taula d''Embotits Catalans","price":14.00,"options":[]},{"name":"Selecció de Formatges","price":13.00,"options":["vegetarian"]},{"name":"Conserves del Dia","price":9.00,"options":[]},{"name":"Torrada amb Sobrassada i Mel","price":6.50,"options":[]},{"name":"Olives Trencades","price":4.00,"options":["vegan","vegetarian"]}]',
    4.2
);

-- Seed ratings (id generated as concat of user+restaurant short ids)
INSERT INTO ratings (id, user_id, restaurant_id, rating) VALUES
    -- alice
    ('b2f60819', 'b2c4d5e6', 'f6081920', 5),
    ('b2071923', 'b2c4d5e6', '07192a31', 5),
    ('b23a4c5d', 'b2c4d5e6', '3a4c5d64', 4),
    ('b27e8091', 'b2c4d5e6', '7e8091a8', 5),
    -- bob
    ('c3071923', 'c3d5e6f7', '07192a31', 4),
    ('c3182a3b', 'c3d5e6f7', '182a3b42', 4),
    ('c34b5d6e', 'c3d5e6f7', '4b5d6e75', 3),
    ('c36d7f80', 'c3d5e6f7', '6d7f8097', 4),
    -- carla
    ('d4293b4c', 'd4e6f708', '293b4c53', 3),
    ('d43a4c5d', 'd4e6f708', '3a4c5d64', 5),
    ('d45c6e7f', 'd4e6f708', '5c6e7f86', 4),
    ('d48f91a2', 'd4e6f708', '8f91a2b9', 4),
    -- david
    ('e5f60819', 'e5f70819', 'f6081920', 4),
    ('e54b5d6e', 'e5f70819', '4b5d6e75', 4),
    ('e57e8091', 'e5f70819', '7e8091a8', 5),
    ('e58f91a2', 'e5f70819', '8f91a2b9', 4);

-- Recompute avg_rating from seed data
UPDATE restaurants r SET avg_rating = (
    SELECT ROUND(AVG(rating)::NUMERIC, 2) FROM ratings WHERE restaurant_id = r.id
) WHERE EXISTS (SELECT 1 FROM ratings WHERE restaurant_id = r.id);
