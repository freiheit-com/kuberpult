import React from 'react';
import { render } from '@testing-library/react';

import { Spinner } from './';

describe('Spinner', () => {
    const getNode = (): JSX.Element | any => {
        // given
        const defaultProps: any = {
            children: null,
        };
        return <Spinner {...defaultProps} />;
    };
    const getWrapper = () => render(getNode());

    it('renders the Spinner component', () => {
        // when
        const { container } = getWrapper();
        // then
        expect(container.querySelector('.MuiCircularProgress-root')).toBeTruthy();
    });
});
