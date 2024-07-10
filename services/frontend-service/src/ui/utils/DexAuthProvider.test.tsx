import { LoginPage, isTokenValid } from './DexAuthProvider';
import { MemoryRouter } from 'react-router-dom';
import { render, renderHook } from '@testing-library/react';
import { fakeLoadEverything } from '../../setupTests';
import {
    UpdateOverview,
} from '../utils/store';

// Mocking document.cookie
Object.defineProperty(document, 'cookie', {
  writable: true,
  value: ''
});

describe('isTokenValid', () => {
  beforeEach(() => {
    // Reset the cookie before each test
    document.cookie = '';
  });

  test('returns false with expired cookie', () => {
    // Dummy token with expiring date on 10, July 2024
    document.cookie = 'kuberpult.oauth=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyLCJleHAiOjE3MjA2MjE5OTd9.6-QS6fw-tEdcmWJP2HNCPzRZaPQgZYwwi5HVoiIX3bo';
    expect(isTokenValid()).toBe(false);
  });

  test('returns false with cookie with no expiring date', () => {
    // Dummy token with no expiring date
    document.cookie = 'kuberpult.oauth=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c';
    expect(isTokenValid()).toBe(false);
  });

  test('returns true with valid cookie', () => {
    // Dummy token with expiring date on year 56494 
    document.cookie = 'kuberpult.oauth=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyLCJleHAiOjE3MjA2MjE5OTc3Nzd9.p3ApN5elnhhRhrh7DCOF-9suPIXYC36Nycf0nHfxuf8';
    expect(isTokenValid()).toBe(true);
  });
});

describe('LoginPage', () => {
    const getNode = (): JSX.Element | any => (
        <MemoryRouter>
            <LoginPage />
        </MemoryRouter>
    );
    const getWrapper = () => render(getNode());

    it('Renders full app', () => {
        fakeLoadEverything(true);
        const { container } = getWrapper();
        expect(container.getElementsByClassName('login-name')[0]).toHaveTextContent('Login Into Dex');
        expect(container.getElementsByClassName('login_button')[0]).toHaveTextContent('Login');
    });
});



