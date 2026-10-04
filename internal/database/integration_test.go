package database_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/nikaydo/reviewer/internal/database"
)

// testStore открывает подключение к тестовой базе.
//
// Тесты интеграционные: им нужна настоящая PostgreSQL, потому что проверяют
// поведение запросов, а не текст SQL. База задаётся переменной
// TEST_DATABASE_URL. Если она не задана, тесты пропускаются — чтобы
// go test ./... проходил без внешних зависимостей.
func testStore(t *testing.T) *database.Store {
	t.Helper()

	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL не задан: интеграционные тесты пропущены")
	}

	ctx := context.Background()
	if err := database.RunMigrations(ctx, url, migrationsDir(t)); err != nil {
		t.Fatalf("миграции не применились: %v", err)
	}

	store, err := database.New(ctx, url)
	if err != nil {
		t.Fatalf("не удалось подключиться к базе: %v", err)
	}
	t.Cleanup(store.Close)
	return store
}

// migrationsDir возвращает путь к каталогу миграций.
func migrationsDir(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("TEST_MIGRATIONS_DIR")
	if dir == "" {
		dir = "../../db/migrations"
	}
	return dir
}

// createUser заводит пользователя и возвращает его идентификатор.
func createUser(t *testing.T, store *database.Store, login string) string {
	t.Helper()

	id, err := store.CreateUser(context.Background(), database.CreateUserParams{
		Login:         login,
		PasswordHash:  "$2a$12$fakehashfakehashfakehashfakehashfakehas",
		Role:          "viewer",
		DefaultPrompt: "test prompt",
		MemoryPrompt:  "test memory prompt",
	})
	if err != nil {
		t.Fatalf("не удалось создать пользователя %s: %v", login, err)
	}
	return id
}

// TestOwnershipEnforcement — главный тест на устранение IDOR.
//
// В исходной версии обработчик удаления и обновления пользовательских
// шаблонов передавал в базу только идентификатор из формы, без проверки
// владельца. Любой авторизованный пользователь мог удалить или изменить чужой
// шаблон, зная его uuid.
//
// Здесь проверяется обратное: операция с ресурсом чужого пользователя должна
// завершаться ErrNotFound и ничего не менять.
func TestOwnershipEnforcement(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	ownerID := createUser(t, store, "owner_user")
	intruderID := createUser(t, store, "intruder_user")

	promptID, err := store.AddCustomPrompt(ctx, ownerID, "мой шаблон", "текст владельца")
	if err != nil {
		t.Fatalf("не удалось создать шаблон: %v", err)
	}

	reviewID, err := store.CreateReview(ctx, database.Review{
		UserID:  ownerID,
		Request: "товар",
		Answer:  "отзыв владельца",
		Model:   "test",
	})
	if err != nil {
		t.Fatalf("не удалось создать отзыв: %v", err)
	}

	t.Run("чужой шаблон нельзя изменить", func(t *testing.T) {
		err := store.UpdateCustomPrompt(ctx, intruderID, promptID, "взлом", "подменённый текст")
		if !errors.Is(err, database.ErrNotFound) {
			t.Fatalf("UpdateCustomPrompt вернул %v, ожидался ErrNotFound", err)
		}

		// Проверяем, что шаблон действительно не изменился.
		prompts, err := store.ListCustomPrompts(ctx, ownerID)
		if err != nil {
			t.Fatalf("ListCustomPrompts: %v", err)
		}
		if len(prompts) != 1 || prompts[0].Prompt != "текст владельца" {
			t.Errorf("шаблон владельца изменён: %+v", prompts)
		}
	})

	t.Run("чужой шаблон нельзя удалить", func(t *testing.T) {
		err := store.DeleteCustomPrompt(ctx, intruderID, promptID)
		if !errors.Is(err, database.ErrNotFound) {
			t.Fatalf("DeleteCustomPrompt вернул %v, ожидался ErrNotFound", err)
		}

		prompts, err := store.ListCustomPrompts(ctx, ownerID)
		if err != nil {
			t.Fatalf("ListCustomPrompts: %v", err)
		}
		if len(prompts) != 1 {
			t.Errorf("шаблон владельца удалён: осталось %d", len(prompts))
		}
	})

	t.Run("чужой отзыв нельзя прочитать", func(t *testing.T) {
		_, err := store.ReviewByID(ctx, intruderID, reviewID)
		if !errors.Is(err, database.ErrNotFound) {
			t.Fatalf("ReviewByID вернул %v, ожидался ErrNotFound", err)
		}
	})

	t.Run("чужой отзыв нельзя изменить", func(t *testing.T) {
		err := store.UpdateReviewAnswer(ctx, intruderID, reviewID, "подменённый отзыв")
		if !errors.Is(err, database.ErrNotFound) {
			t.Fatalf("UpdateReviewAnswer вернул %v, ожидался ErrNotFound", err)
		}

		review, err := store.ReviewByID(ctx, ownerID, reviewID)
		if err != nil {
			t.Fatalf("ReviewByID: %v", err)
		}
		if review.Answer != "отзыв владельца" {
			t.Errorf("ответ отзыва изменён: %q", review.Answer)
		}
	})

	t.Run("чужая метка избранного не меняется", func(t *testing.T) {
		err := store.SetReviewFavorite(ctx, intruderID, reviewID, true)
		if !errors.Is(err, database.ErrNotFound) {
			t.Fatalf("SetReviewFavorite вернул %v, ожидался ErrNotFound", err)
		}

		review, err := store.ReviewByID(ctx, ownerID, reviewID)
		if err != nil {
			t.Fatalf("ReviewByID: %v", err)
		}
		if review.Favorite {
			t.Error("отметка избранного чужого отзыва была установлена")
		}
	})

	t.Run("чужой отзыв нельзя удалить", func(t *testing.T) {
		err := store.DeleteReview(ctx, intruderID, reviewID)
		if !errors.Is(err, database.ErrNotFound) {
			t.Fatalf("DeleteReview вернул %v, ожидался ErrNotFound", err)
		}
	})

	t.Run("чужой память нельзя прочитать", func(t *testing.T) {
		if err := store.RememberMemory(ctx, ownerID, "любит кофе"); err != nil {
			t.Fatalf("RememberMemory: %v", err)
		}

		got, err := store.RecallMemory(ctx, intruderID)
		if err != nil {
			t.Fatalf("RecallMemory: %v", err)
		}
		// У нарушителя нет записи памяти, поэтому результат пустой.
		if got != "" {
			t.Errorf("прочитана чужая память: %q", got)
		}
	})
}

func TestRefreshTokenRotationDetectsReuse(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	userID := createUser(t, store, "rotate_user")

	hash1 := "hash-of-first-token"
	familyID := "family-1"
	expiry := time.Now().Add(time.Hour)

	if err := store.SaveRefreshToken(ctx, userID, familyID, hash1, expiry); err != nil {
		t.Fatalf("SaveRefreshToken: %v", err)
	}

	// Ротация: первый токен заменяется вторым.
	hash2 := "hash-of-second-token"
	if err := store.RotateRefreshToken(ctx, userID, familyID, hash1, hash2, expiry); err != nil {
		t.Fatalf("RotateRefreshToken: %v", err)
	}

	// По новому токену пользователь находится.
	if _, err := store.UserByRefreshHash(ctx, hash2); err != nil {
		t.Fatalf("UserByRefreshHash не нашёл активный токен: %v", err)
	}

	// Повторное предъявление старого токена — признак утечки.
	err := store.RotateRefreshToken(ctx, userID, familyID, hash1, "hash-of-third-token", expiry)
	var reused *database.ErrRefreshReused
	if !errors.As(err, &reused) {
		t.Fatalf("RotateRefreshToken вернул %v, ожидалась ErrRefreshReused", err)
	}
}

func TestRevokedTokenIsNotFound(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	userID := createUser(t, store, "revoke_user")
	hash := "hash-to-revoke"
	expiry := time.Now().Add(time.Hour)

	if err := store.SaveRefreshToken(ctx, userID, "family-revoke", hash, expiry); err != nil {
		t.Fatalf("SaveRefreshToken: %v", err)
	}
	if err := store.RevokeRefreshToken(ctx, userID, hash); err != nil {
		t.Fatalf("RevokeRefreshToken: %v", err)
	}

	if _, err := store.UserByRefreshHash(ctx, hash); !errors.Is(err, database.ErrNotFound) {
		t.Fatalf("отозванный токен всё ещё находится: %v", err)
	}
}

func TestExpiredTokenIsNotFound(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	userID := createUser(t, store, "expired_user")
	hash := "already-expired"

	// Срок в прошлом: токен сразу считается истёкшим.
	if err := store.SaveRefreshToken(ctx, userID, "family-expired", hash, time.Now().Add(-time.Hour)); err != nil {
		t.Fatalf("SaveRefreshToken: %v", err)
	}

	if _, err := store.UserByRefreshHash(ctx, hash); !errors.Is(err, database.ErrNotFound) {
		t.Fatalf("истёкший токен всё ещё находится: %v", err)
	}
}

func TestRevokeAllUserTokens(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	userID := createUser(t, store, "logout_user")
	expiry := time.Now().Add(time.Hour)

	for i, hash := range []string{"session-a", "session-b"} {
		family := "family-" + hash
		if err := store.SaveRefreshToken(ctx, userID, family, hash, expiry); err != nil {
			t.Fatalf("SaveRefreshToken %d: %v", i, err)
		}
	}

	if err := store.RevokeAllUserTokens(ctx, userID); err != nil {
		t.Fatalf("RevokeAllUserTokens: %v", err)
	}

	for _, hash := range []string{"session-a", "session-b"} {
		if _, err := store.UserByRefreshHash(ctx, hash); !errors.Is(err, database.ErrNotFound) {
			t.Errorf("токен %s не отозван: %v", hash, err)
		}
	}
}

func TestCreateUserRejectsDuplicateLogin(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	createUser(t, store, "duplicate_login")

	_, err := store.CreateUser(ctx, database.CreateUserParams{
		Login:         "duplicate_login",
		PasswordHash:  "$2a$12$anotherhashanotherhashanotherhashanotherhash",
		Role:          "viewer",
		DefaultPrompt: "p",
		MemoryPrompt:  "m",
	})
	if !errors.Is(err, database.ErrLoginTaken) {
		t.Fatalf("CreateUser вернул %v, ожидался ErrLoginTaken", err)
	}
}

func TestCreateUserRejectsInvalidLogin(t *testing.T) {
	store := testStore(t)

	for _, login := range []string{"", "ab", "с пробелом"} {
		t.Run(login, func(t *testing.T) {
			_, err := store.CreateUser(context.Background(), database.CreateUserParams{
				Login:        login,
				PasswordHash: "$2a$12$hash",
				Role:         "viewer",
			})
			if err == nil {
				t.Fatalf("логин %q должен отклоняться", login)
			}
		})
	}
}

func TestMissingPresetDoesNotFailReview(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	userID := createUser(t, store, "preset_user")

	// Раньше отсутствие выбранного пресета приводило к ошибке, и запрос
	// отзыва падал, даже если пресет был не нужен.
	main, preset, err := store.PromptsForReview(ctx, userID, "")
	if err != nil {
		t.Fatalf("PromptsForReview вернул ошибку при пустом пресете: %v", err)
	}
	if preset != "" {
		t.Errorf("пресет = %q, ожидалась пустая строка", preset)
	}
	if main == "" {
		t.Error("основной промт не вернулся")
	}
}

func TestForeignPresetIsIgnored(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	ownerID := createUser(t, store, "preset_owner")
	intruderID := createUser(t, store, "preset_intruder")

	presetID, err := store.AddCustomPrompt(ctx, ownerID, "критерий", "только факты")
	if err != nil {
		t.Fatalf("AddCustomPrompt: %v", err)
	}

	// Пресет чужого пользователя не должен подставляться: это раскрыло бы
	// содержимое чужого шаблона в системном промте.
	_, preset, err := store.PromptsForReview(ctx, intruderID, presetID)
	if err != nil {
		t.Fatalf("PromptsForReview вернул ошибку: %v", err)
	}
	if preset != "" {
		t.Errorf("чужой пресет подставлен: %q", preset)
	}
}

func TestRecallMemoryMissingIsEmpty(t *testing.T) {
	store := testStore(t)
	userID := createUser(t, store, "nomemory_user")

	// У пользователя без записи памяти генерация должна работать.
	got, err := store.RecallMemory(context.Background(), userID)
	if err != nil {
		t.Fatalf("RecallMemory вернул ошибку: %v", err)
	}
	if got != "" {
		t.Errorf("память = %q, ожидалась пустая строка", got)
	}
}

func TestListReviewsReturnsTitlesWithoutDuplicates(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	userID := createUser(t, store, "titles_user")

	reviewID, err := store.CreateReview(ctx, database.Review{
		UserID:  userID,
		Request: "товар",
		Answer:  "ответ",
		Model:   "test",
	})
	if err != nil {
		t.Fatalf("CreateReview: %v", err)
	}

	// Заголовок добавляется повторно: запрос должен обновлять существующий,
	// а не плодить дубликаты. Наивный LEFT JOIN возвращал бы отзыв дважды.
	for i := 0; i < 3; i++ {
		if err := store.UpsertReviewTitle(ctx, userID, reviewID, "Заголовок", "товар"); err != nil {
			t.Fatalf("UpsertReviewTitle %d: %v", i, err)
		}
	}

	reviews, err := store.ListReviews(ctx, userID)
	if err != nil {
		t.Fatalf("ListReviews: %v", err)
	}
	if len(reviews) != 1 {
		t.Fatalf("отзывов %d, ожидался 1", len(reviews))
	}
	if reviews[0].Title.Text == nil || *reviews[0].Title.Text != "Заголовок" {
		t.Errorf("заголовок = %+v", reviews[0].Title)
	}
}
