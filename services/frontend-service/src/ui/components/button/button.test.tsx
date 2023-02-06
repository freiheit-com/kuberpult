import { Button } from './button';
import { render } from '@testing-library/react';

describe('Button', () => {
    const getNode = () => <Button className={'button--test'} label={'Test Me'} />;
    const getWrapper = () => render(getNode());
    it('renders a button', () => {
        const { container } = getWrapper();
        expect(container.firstChild).toMatchInlineSnapshot(`
    <button
      aria-label="Test Me"
      class="mdc-button button--test"
    >
      <div
        class="mdc-button__ripple"
      />
      <span
        class="mdc-button__label"
      >
        Test Me
      </span>
    </button>
  `);
    });
});
