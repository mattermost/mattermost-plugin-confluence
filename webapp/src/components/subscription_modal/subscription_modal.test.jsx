/*eslint max-nested-callbacks: ["error", 3]*/

import React from 'react';

import {shallow} from 'enzyme';

import Constants from '../../constants';

import SubscriptionModal from './subscription_modal';

describe('components/ChannelSettingsModal', () => {
    const baseProps = {
        theme: {},
        visibility: false,
        subscription: {},
        close: jest.fn(),
        saveChannelSubscription: jest.fn().mockResolvedValue({}),
        currentChannelID: 'abcabcabcabcabc',
        editChannelSubscription: jest.fn().mockResolvedValue({}),
        getPluginConfig: jest.fn().mockResolvedValue({
            data: {
                supportedEvents: Constants.CONFLUENCE_EVENTS,
            },
        }),
    };

    test('subscription modal snapshot test', async () => {
        const props = {
            ...baseProps,
            visibility: true,
        };
        const wrapper = shallow(
            <SubscriptionModal {...props}/>,
        );
        expect(wrapper).toMatchSnapshot();
    });

    test('new space subscription', async () => {
        const props = {
            ...baseProps,
            visibility: true,
        };
        const wrapper = shallow(
            <SubscriptionModal {...props}/>,
        );
        wrapper.setState({
            alias: 'Abc',
            baseURL: 'https://test.com',
            spaceKey: 'test',
            events: Constants.CONFLUENCE_EVENTS,
            error: '',
            saving: false,
            pageID: '',
            subscriptionType: Constants.SUBSCRIPTION_TYPE[0],
        });
        wrapper.instance().handleSubmit({preventDefault: jest.fn()});
        expect(wrapper.state().error).toBe('');
        expect(props.saveChannelSubscription).toHaveBeenCalledWith({
            alias: 'Abc',
            baseURL: 'https://test.com',
            spaceKey: 'test',
            events: Constants.CONFLUENCE_EVENTS.map((event) => event.value),
            channelID: 'abcabcabcabcabc',
            pageID: '',
            subscriptionType: 'space_subscription',
        });

        expect(props.editChannelSubscription).not.toHaveBeenCalled();
    });

    test('edit space subscription', async () => {
        const subscription = {
            alias: 'Abc',
            baseURL: 'https://test.com',
            spaceKey: 'test',
            events: Constants.CONFLUENCE_EVENTS,
            pageID: '',
            subscriptionType: Constants.SUBSCRIPTION_TYPE[0].value,
        };
        const props = {
            ...baseProps,
            subscription,
        };
        const wrapper = shallow(
            <SubscriptionModal {...props}/>,
        );
        wrapper.setState({
            oldAlias: 'Abc',
            alias: 'Xyz',
            baseURL: 'https://test.com',
            spaceKey: 'test',
            events: Constants.CONFLUENCE_EVENTS,
            error: '',
            saving: false,
            pageID: '',
            subscriptionType: Constants.SUBSCRIPTION_TYPE[0],
        });
        wrapper.instance().handleSubmit({preventDefault: jest.fn()});
        expect(wrapper.state().error).toBe('');
        expect(props.editChannelSubscription).toHaveBeenCalledWith({
            oldAlias: 'Abc',
            alias: 'Xyz',
            baseURL: 'https://test.com',
            spaceKey: 'test',
            events: Constants.CONFLUENCE_EVENTS.map((event) => event.value),
            channelID: 'abcabcabcabcabc',
            pageID: '',
            subscriptionType: 'space_subscription',
        });
        expect(props.saveChannelSubscription).not.toHaveBeenCalled();
    });

    test('new page subscription', async () => {
        const props = {
            ...baseProps,
            visibility: true,
        };
        const wrapper = shallow(
            <SubscriptionModal {...props}/>,
        );
        wrapper.setState({
            alias: 'Abc',
            baseURL: 'https://test.com',
            spaceKey: '',
            events: Constants.CONFLUENCE_EVENTS,
            error: '',
            saving: false,
            pageID: '1234',
            subscriptionType: Constants.SUBSCRIPTION_TYPE[1],
        });
        wrapper.instance().handleSubmit({preventDefault: jest.fn()});
        expect(wrapper.state().error).toBe('');
        expect(props.saveChannelSubscription).toHaveBeenCalledWith({
            alias: 'Abc',
            baseURL: 'https://test.com',
            spaceKey: '',
            events: Constants.CONFLUENCE_EVENTS.map((event) => event.value),
            channelID: 'abcabcabcabcabc',
            pageID: '1234',
            subscriptionType: 'page_subscription',
        });

        expect(props.editChannelSubscription).not.toHaveBeenCalled();
    });

    test('edit page subscription', async () => {
        const subscription = {
            oldAlias: 'Abc',
            alias: 'Abc',
            baseURL: 'https://test.com',
            spaceKey: 'test',
            events: Constants.CONFLUENCE_EVENTS,
            pageID: '1234',
            subscriptionType: Constants.SUBSCRIPTION_TYPE[1].value,
        };
        const props = {
            ...baseProps,
            subscription,
        };
        const wrapper = shallow(
            <SubscriptionModal {...props}/>,
        );
        wrapper.setState({
            oldAlias: 'Abc',
            alias: 'Xyz',
            baseURL: 'https://test.com',
            spaceKey: '',
            events: Constants.CONFLUENCE_EVENTS,
            error: '',
            saving: false,
            pageID: '1234',
            subscriptionType: Constants.SUBSCRIPTION_TYPE[1],
        });
        wrapper.instance().handleSubmit({preventDefault: jest.fn()});
        expect(wrapper.state().error).toBe('');
        expect(props.editChannelSubscription).toHaveBeenCalledWith({
            oldAlias: 'Abc',
            alias: 'Xyz',
            baseURL: 'https://test.com',
            spaceKey: '',
            events: Constants.CONFLUENCE_EVENTS.map((event) => event.value),
            channelID: 'abcabcabcabcabc',
            pageID: '1234',
            subscriptionType: 'page_subscription',
        });
        expect(props.saveChannelSubscription).not.toHaveBeenCalled();
    });

    test('subscription data clean', async () => {
        const props = {
            ...baseProps,
            visibility: true,
        };
        const wrapper = shallow(
            <SubscriptionModal {...props}/>,
        );
        wrapper.setState({
            alias: '   Abc   ',
            baseURL: 'https://teST.com',
            spaceKey: 'test       ',
            events: Constants.CONFLUENCE_EVENTS,
            error: '',
            saving: false,
            pageID: '',
            subscriptionType: Constants.SUBSCRIPTION_TYPE[0],
        });
        wrapper.instance().handleSubmit({preventDefault: jest.fn()});
        expect(wrapper.state().error).toBe('');
        expect(props.saveChannelSubscription).toHaveBeenCalledWith({
            alias: 'Abc',
            baseURL: 'https://test.com',
            spaceKey: 'test',
            events: Constants.CONFLUENCE_EVENTS.map((event) => event.value),
            channelID: 'abcabcabcabcabc',
            pageID: '',
            subscriptionType: 'space_subscription',
        });

        expect(props.editChannelSubscription).not.toHaveBeenCalled();
    });

    test('loads plugin config on mount', async () => {
        const wrapper = shallow(
            <SubscriptionModal {...baseProps}/>,
        );
        await wrapper.instance().componentDidMount();
        wrapper.update();
        expect(baseProps.getPluginConfig).toHaveBeenCalled();
        expect(wrapper.state().supportedEvents).toEqual(Constants.CONFLUENCE_EVENTS);
    });
});
