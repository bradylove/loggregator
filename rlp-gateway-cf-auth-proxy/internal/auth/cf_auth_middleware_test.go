package auth_test

import (
	"net/http"
	"net/http/httptest"

	"code.cloudfoundry.org/loggregator/rlp-gateway-cf-auth-proxy/internal/auth"

	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CfAuthMiddleware", func() {
	var (
		spyOauth2ClientReader *spyOauth2ClientReader
		spyLogAuthorizer      *spyLogAuthorizer

		recorder *httptest.ResponseRecorder
		request  *http.Request
		provider auth.CFAuthMiddlewareProvider
	)

	BeforeEach(func() {
		spyOauth2ClientReader = newAdminChecker()
		spyLogAuthorizer = newSpyLogAuthorizer()

		provider = auth.NewCFAuthMiddlewareProvider(
			spyOauth2ClientReader,
			spyLogAuthorizer,
		)

		recorder = httptest.NewRecorder()
	})

	Describe("/v2/read", func() {
		BeforeEach(func() {
			request = httptest.NewRequest(http.MethodGet, "/v2/read?source_id=123", nil)
		})

		It("forwards the /v2/read request to the handler if user is an admin", func() {
			var baseHandlerCalled bool
			baseHandler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				baseHandlerCalled = true
			})
			authHandler := provider.Middleware(baseHandler)

			spyOauth2ClientReader.result = true

			request.Header.Set("Authorization", "bearer valid-token")

			authHandler.ServeHTTP(recorder, request)

			Expect(recorder.Code).To(Equal(http.StatusOK))
			Expect(baseHandlerCalled).To(BeTrue())

			Expect(spyOauth2ClientReader.token).To(Equal("bearer valid-token"))
		})

		It("forwards the /v2/read request to the handler if non-admin user has log access", func() {
			spyLogAuthorizer.result = true
			var baseHandlerCalled bool
			baseHandler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				baseHandlerCalled = true
			})
			authHandler := provider.Middleware(baseHandler)

			request.Header.Set("Authorization", "valid-token")

			// Call result
			authHandler.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusOK))
			Expect(baseHandlerCalled).To(BeTrue())

			//verify CAPI called with correct info
			Expect(spyLogAuthorizer.token).To(Equal("valid-token"))
			Expect(spyLogAuthorizer.sourceID).To(Equal("123"))
		})

		It("returns 404 if there's no authorization header present", func() {
			var baseHandlerCalled bool
			baseHandler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				baseHandlerCalled = true
			})
			authHandler := provider.Middleware(baseHandler)

			authHandler.ServeHTTP(recorder, request)

			Expect(recorder.Code).To(Equal(http.StatusNotFound))
			Expect(baseHandlerCalled).To(BeFalse())
		})

		It("returns 404 if Oauth2ClientReader returns an error", func() {
			var baseHandlerCalled bool
			baseHandler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				baseHandlerCalled = true
			})
			authHandler := provider.Middleware(baseHandler)

			spyOauth2ClientReader.err = errors.New("some-error")
			spyOauth2ClientReader.result = true
			spyLogAuthorizer.result = true

			request.Header.Set("Authorization", "valid-token")
			authHandler.ServeHTTP(recorder, request)

			Expect(recorder.Code).To(Equal(http.StatusNotFound))
			Expect(baseHandlerCalled).To(BeFalse())
		})

		It("returns 404 if user is not authorized", func() {
			var baseHandlerCalled bool
			baseHandler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				baseHandlerCalled = true
			})
			authHandler := provider.Middleware(baseHandler)

			spyOauth2ClientReader.result = false
			spyLogAuthorizer.result = false

			request.Header.Set("Authorization", "valid-token")
			authHandler.ServeHTTP(recorder, request)

			Expect(recorder.Code).To(Equal(http.StatusNotFound))
			Expect(baseHandlerCalled).To(BeFalse())
		})
	})
})

type spyOauth2ClientReader struct {
	token  string
	result bool
	client string
	user   string
	err    error
}

func newAdminChecker() *spyOauth2ClientReader {
	return &spyOauth2ClientReader{}
}

func (s *spyOauth2ClientReader) Read(token string) (auth.Oauth2Client, error) {
	s.token = token
	return auth.Oauth2Client{
		IsAdmin:  s.result,
		ClientID: s.client,
		UserID:   s.user,
	}, s.err
}

type spyLogAuthorizer struct {
	result          bool
	sourceID        string
	token           string
	available       []string
	availableCalled int
}

func newSpyLogAuthorizer() *spyLogAuthorizer {
	return &spyLogAuthorizer{}
}

func (s *spyLogAuthorizer) IsAuthorized(sourceID, token string) bool {
	s.sourceID = sourceID
	s.token = token
	return s.result
}

func (s *spyLogAuthorizer) AvailableSourceIDs(token string) []string {
	s.availableCalled++
	s.token = token
	return s.available
}
