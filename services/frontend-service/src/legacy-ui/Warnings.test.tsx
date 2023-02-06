import React from 'react';
import { getByLabelText, render } from '@testing-library/react';
import { UndeployBtn } from './Warnings';

describe('Undeploy Button', () => {
    interface dataT {
        name: string;
        inCart?: boolean;
        selector: (container: HTMLElement) => HTMLElement | null;
    }

    const data: dataT[] = [
        {
            name: 'renders the UndeployBtn component',
            inCart: false,
            selector: (container) => getByLabelText(container, /This app is ready to un-deploy./i),
        },
        {
            name: 'renders the UndeployBtn component with resolved state',
            inCart: true,
            selector: (container) => container.querySelector('.Mui-disabled'),
        },
    ];

    const getNode = (overrides?: {}): JSX.Element | any => {
        // given
        const defaultProps: any = {
            children: null,
            inCart: false, //
            applicationName: 'app1', //
        };
        return <UndeployBtn {...defaultProps} {...overrides} />;
    };
    const getWrapper = (overrides?: { inCart?: boolean }) => render(getNode(overrides));

    describe.each(data)(`Undeploy Button with state`, (testcase) => {
        it(testcase.name, () => {
            // when
            const { container } = getWrapper({ inCart: testcase.inCart });
            // then
            expect(testcase.selector(container)).toBeTruthy();
        });
    });
});
